/// The debug overlay that can be drawn on top of the terminal
/// during the rendering process.
///
/// This is implemented by doing all the drawing on the CPU via z2d,
/// since the debug overlay isn't that common, z2d is pretty fast, and
/// it simplifies our implementation quite a bit by not relying on us
/// having a bunch of shaders that we have to write per-platform.
///
/// Initialize the overlay, apply features with `applyFeatures`, then
/// get the resulting image with `pendingImage` to upload to the GPU.
/// This works in concert with `renderer.image.State` to simplify. Draw
/// it on the GPU as an image composited on top of the terminal output.
const Overlay = @This();

const std = @import("std");
const Allocator = std.mem.Allocator;
const z2d = @import("z2d");
const terminal = @import("../terminal/main.zig");
const size = @import("size.zig");
const Size = size.Size;
const CellSize = size.CellSize;
const Image = @import("image.zig").Image;

const log = std.log.scoped(.renderer_overlay);
const matrix9180_cell_scale_x: f32 = 0.38;
const matrix9180_cell_scale_y: f32 = 0.34;
const matrix9180_dot_peak: u8 = 148;
const matrix9180_dot_edge_feather = 0.74;

/// The colors we use for overlays.
pub const Color = enum {
    hyperlink, // light blue
    semantic_prompt, // orange/gold
    semantic_input, // cyan

    pub fn rgb(self: Color) z2d.pixel.RGB {
        return switch (self) {
            .hyperlink => .{ .r = 180, .g = 180, .b = 255 },
            .semantic_prompt => .{ .r = 255, .g = 200, .b = 64 },
            .semantic_input => .{ .r = 64, .g = 200, .b = 255 },
        };
    }

    /// The fill color for rectangles.
    pub fn rectFill(self: Color) z2d.Pixel {
        return self.alphaPixel(96);
    }

    /// The border color for rectangles.
    pub fn rectBorder(self: Color) z2d.Pixel {
        return self.alphaPixel(200);
    }

    /// The raw RGB as a pixel.
    pub fn pixel(self: Color) z2d.Pixel {
        return self.rgb().asPixel();
    }

    fn alphaPixel(self: Color, alpha: u8) z2d.Pixel {
        var rgba: z2d.pixel.RGBA = .fromPixel(self.pixel());
        rgba.a = alpha;
        return rgba.multiply().asPixel();
    }
};

/// The surface we're drawing our overlay to.
surface: z2d.Surface,

/// Cell size information so we can map grid coordinates to pixels.
cell_size: CellSize,

/// The set of available features and their configuration.
pub const Feature = union(enum) {
    highlight_hyperlinks,
    semantic_prompts,
};

pub const InitError = Allocator.Error || error{
    // The terminal dimensions are invalid to support an overlay.
    // Either too small or too big.
    InvalidDimensions,
};

/// Initialize a new, blank overlay.
pub fn init(alloc: Allocator, sz: Size) InitError!Overlay {
    // Our surface does NOT need to take into account padding because
    // we render the overlay using the image subsystem and shaders which
    // already take that into account.
    const term_size = sz.terminal();
    var sfc = z2d.Surface.initPixel(
        .{ .rgba = .{ .r = 0, .g = 0, .b = 0, .a = 0 } },
        alloc,
        std.math.cast(i32, term_size.width) orelse
            return error.InvalidDimensions,
        std.math.cast(i32, term_size.height) orelse
            return error.InvalidDimensions,
    ) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
        error.InvalidWidth, error.InvalidHeight => return error.InvalidDimensions,
    };
    errdefer sfc.deinit(alloc);

    return .{
        .surface = sfc,
        .cell_size = sz.cell,
    };
}

pub fn deinit(self: *Overlay, alloc: Allocator) void {
    self.surface.deinit(alloc);
}

/// Returns a pending image that can be used to copy, convert, upload, etc.
pub fn pendingImage(self: *const Overlay) Image.Pending {
    return .{
        .width = @intCast(self.surface.getWidth()),
        .height = @intCast(self.surface.getHeight()),
        .pixel_format = .rgba,
        .data = @ptrCast(self.surface.image_surface_rgba.buf.ptr),
    };
}

/// Clear the overlay.
pub fn reset(self: *Overlay) void {
    self.surface.paintPixel(.{ .rgba = .{
        .r = 0,
        .g = 0,
        .b = 0,
        .a = 0,
    } });
}

/// Apply the given features to this overlay. This will draw on top of
/// any pre-existing content in the overlay.
pub fn applyFeatures(
    self: *Overlay,
    alloc: Allocator,
    state: *const terminal.RenderState,
    features: []const Feature,
) void {
    self.drawMatrix9180(&state.matrix9180);
    if (state.matrix9180.layers.items.len > 0) {
        self.postProcessMatrix9180(alloc) catch |err| {
            log.warn("matrix9180 post-process failed: {}", .{err});
        };
    }

    for (features) |f| switch (f) {
        .highlight_hyperlinks => self.highlightHyperlinks(
            alloc,
            state,
        ),
        .semantic_prompts => self.highlightSemanticPrompts(
            alloc,
            state,
        ),
    };
}

fn drawMatrix9180(
    self: *Overlay,
    frame: *const terminal.matrix9180.Frame,
) void {
    const cols = @divTrunc(
        @as(usize, @intCast(self.surface.getWidth())),
        matrixCellWidthPixels(self),
    );
    const rows = @divTrunc(
        @as(usize, @intCast(self.surface.getHeight())),
        matrixCellHeightPixels(self),
    );
    log.debug(
        "matrix9180 overlay surface={}x{} cell={}x{} grid={}x{} layers={}",
        .{
            self.surface.getWidth(),
            self.surface.getHeight(),
            self.cell_size.width,
            self.cell_size.height,
            cols,
            rows,
            frame.layers.items.len,
        },
    );

    for (frame.layers.items) |layer| {
        const channel = switch (layer.id) {
            1 => Channel.red,
            2 => Channel.green,
            3 => Channel.blue,
            else => continue,
        };

        var view = std.unicode.Utf8View.init(layer.data) catch continue;
        var it = view.iterator();
        var x: i32 = layer.x_offset;
        var y: i32 = layer.y_offset;
        var rendered_cells: usize = 0;
        var clipped_cells: usize = 0;
        var newline_count: usize = 0;
        var max_x: i32 = x;
        var max_y: i32 = y;
        while (it.nextCodepoint()) |cp| {
            switch (cp) {
                '\n' => {
                    newline_count += 1;
                    y += 1;
                    x = layer.x_offset;
                    max_y = @max(max_y, y);
                },

                else => {
                    if (cp >= 0x2800 and cp <= 0x28FF) {
                        if (self.drawBraillePattern(
                            x,
                            y,
                            @intCast(cp - 0x2800),
                            channel,
                            layer.alpha,
                        )) {
                            rendered_cells += 1;
                        } else {
                            clipped_cells += 1;
                        }
                    }
                    max_x = @max(max_x, x);
                    max_y = @max(max_y, y);
                    x += 1;
                },
            }
        }

        log.debug(
            "matrix9180 overlay layer id={} z={} offset=({}, {}) bytes={} rows={} max_cols={} rendered_cells={} clipped_cells={} newlines={} extent=({}, {})",
            .{
                layer.id,
                layer.z_index,
                layer.x_offset,
                layer.y_offset,
                layer.data.len,
                layer.rowCount(),
                layer.maxCols(),
                rendered_cells,
                clipped_cells,
                newline_count,
                max_x,
                max_y,
            },
        );
    }
}

const Channel = enum {
    red,
    green,
    blue,
};

fn drawBraillePattern(
    self: *Overlay,
    grid_x: i32,
    grid_y: i32,
    pattern: u8,
    channel: Channel,
    alpha: u8,
) bool {
    if (pattern == 0) return false;
    if (grid_x < 0 or grid_y < 0) return false;

    const cell_x: usize = @intCast(grid_x);
    const cell_y: usize = @intCast(grid_y);
    const cols = @divTrunc(
        @as(usize, @intCast(self.surface.getWidth())),
        matrixCellWidthPixels(self),
    );
    const rows = @divTrunc(
        @as(usize, @intCast(self.surface.getHeight())),
        matrixCellHeightPixels(self),
    );
    if (cell_x >= cols or cell_y >= rows) return false;

    const dot_map = [_][2]u8{
        .{ 0, 0 },
        .{ 0, 1 },
        .{ 0, 2 },
        .{ 1, 0 },
        .{ 1, 1 },
        .{ 1, 2 },
        .{ 0, 3 },
        .{ 1, 3 },
    };

    for (dot_map, 0..) |dot, idx| {
        if ((pattern & (@as(u8, 1) << @intCast(idx))) == 0) continue;
        self.drawBrailleDot(cell_x, cell_y, dot[0], dot[1], channel, alpha);
    }

    return true;
}

fn drawBrailleDot(
    self: *Overlay,
    cell_x: usize,
    cell_y: usize,
    dot_col: u8,
    dot_row: u8,
    channel: Channel,
    alpha: u8,
) void {
    const cell_w = matrixCellWidth(self);
    const cell_h = matrixCellHeight(self);

    const base_x = @as(f32, @floatFromInt(cell_x)) * cell_w;
    const base_y = @as(f32, @floatFromInt(cell_y)) * cell_h;
    const bucket_x0 = @as(usize, @intFromFloat(@floor(base_x + (@as(f32, @floatFromInt(dot_col)) * cell_w) / 2)));
    const bucket_x1 = @as(usize, @intFromFloat(@ceil(base_x + (@as(f32, @floatFromInt(dot_col + 1)) * cell_w) / 2)));
    const bucket_y0 = @as(usize, @intFromFloat(@floor(base_y + (@as(f32, @floatFromInt(dot_row)) * cell_h) / 4)));
    const bucket_y1 = @as(usize, @intFromFloat(@ceil(base_y + (@as(f32, @floatFromInt(dot_row + 1)) * cell_h) / 4)));
    if (bucket_x0 >= bucket_x1 or bucket_y0 >= bucket_y1) return;

    const bucket_w = bucket_x1 - bucket_x0;
    const bucket_h = bucket_y1 - bucket_y0;
    const pad_x = bucket_w / 6;
    const pad_y = bucket_h / 6;
    const dot_x0 = bucket_x0 + @min(pad_x, bucket_w - 1);
    const dot_y0 = bucket_y0 + @min(pad_y, bucket_h - 1);
    const dot_x1 = bucket_x1 - @min(pad_x, bucket_w - 1);
    const dot_y1 = bucket_y1 - @min(pad_y, bucket_h - 1);
    if (dot_x0 >= dot_x1 or dot_y0 >= dot_y1) return;

    self.addChannelDot(dot_x0, dot_y0, dot_x1, dot_y1, channel, alpha);
}

fn matrixCellWidth(self: *const Overlay) f32 {
    return @max(@as(f32, @floatFromInt(self.cell_size.width)) * matrix9180_cell_scale_x, 2.0);
}

fn matrixCellHeight(self: *const Overlay) f32 {
    return @max(@as(f32, @floatFromInt(self.cell_size.height)) * matrix9180_cell_scale_y, 4.0);
}

fn matrixCellWidthPixels(self: *const Overlay) usize {
    return @as(usize, @intFromFloat(@ceil(matrixCellWidth(self))));
}

fn matrixCellHeightPixels(self: *const Overlay) usize {
    return @as(usize, @intFromFloat(@ceil(matrixCellHeight(self))));
}

fn addChannelDot(
    self: *Overlay,
    x0: usize,
    y0: usize,
    x1: usize,
    y1: usize,
    channel: Channel,
    alpha: u8,
) void {
    const width: usize = @intCast(self.surface.getWidth());
    const height: usize = @intCast(self.surface.getHeight());
    const max_x = @min(x1, width);
    const max_y = @min(y1, height);
    if (x0 >= max_x or y0 >= max_y) return;

    const buf = self.surface.image_surface_rgba.buf;
    const span_w = @as(f32, @floatFromInt(max_x - x0));
    const span_h = @as(f32, @floatFromInt(max_y - y0));
    const center_x = @as(f32, @floatFromInt(x0)) + span_w / 2;
    const center_y = @as(f32, @floatFromInt(y0)) + span_h / 2;
    const radius_x = @max(span_w / 2, 0.5);
    const radius_y = @max(span_h / 2, 0.5);

    for (y0..max_y) |y| {
        for (x0..max_x) |x| {
            const px_center_x = @as(f32, @floatFromInt(x)) + 0.5;
            const px_center_y = @as(f32, @floatFromInt(y)) + 0.5;
            const norm_x = (px_center_x - center_x) / radius_x;
            const norm_y = (px_center_y - center_y) / radius_y;
            const dist = @sqrt(norm_x * norm_x + norm_y * norm_y);
            const coverage = smoothstep(1.0, matrix9180_dot_edge_feather, dist);
            if (coverage <= 0) continue;

            const scaled_peak = @as(f32, @floatFromInt(matrix9180_dot_peak)) *
                (@as(f32, @floatFromInt(alpha)) / 255.0);
            const contribution = @as(u8, @intFromFloat(@round(scaled_peak * coverage)));
            if (contribution == 0) continue;

            const idx = y * width + x;
            var px = &buf[idx];
            switch (channel) {
                .red => px.r = saturatingAdd(px.r, contribution),
                .green => px.g = saturatingAdd(px.g, contribution),
                .blue => px.b = saturatingAdd(px.b, contribution),
            }
            px.a = @max(px.a, @max(px.r, @max(px.g, px.b)));
        }
    }
}

fn smoothstep(edge0: f32, edge1: f32, x: f32) f32 {
    if (edge0 == edge1) return if (x < edge0) 0 else 1;

    const low = @min(edge0, edge1);
    const high = @max(edge0, edge1);
    const t = std.math.clamp((x - low) / (high - low), 0, 1);
    const curve = t * t * (3 - 2 * t);
    return if (edge0 < edge1) curve else 1 - curve;
}

fn saturatingAdd(a: u8, b: u8) u8 {
    const sum: u16 = @as(u16, a) + @as(u16, b);
    return @intCast(@min(sum, std.math.maxInt(u8)));
}

fn postProcessMatrix9180(self: *Overlay, alloc: Allocator) !void {
    const width: usize = @intCast(self.surface.getWidth());
    const height: usize = @intCast(self.surface.getHeight());
    if (width == 0 or height == 0) return;

    const Pixel = @TypeOf(self.surface.image_surface_rgba.buf[0]);
    const buf = self.surface.image_surface_rgba.buf;
    const scratch = try alloc.alloc(Pixel, buf.len);
    defer alloc.free(scratch);

    @memcpy(scratch, buf);

    for (0..height) |y| {
        for (0..width) |x| {
            const orig = scratch[y * width + x];
            const blur = gaussian5x3(scratch, width, height, x, y);

            var out = &buf[y * width + x];
            out.r = combineMatrixChannel(orig.r, blur.r);
            out.g = combineMatrixChannel(orig.g, blur.g);
            out.b = combineMatrixChannel(orig.b, blur.b);
            out.a = @max(out.a, @max(out.r, @max(out.g, out.b)));
        }
    }
}

fn gaussian5x3(
    src: anytype,
    width: usize,
    height: usize,
    x: usize,
    y: usize,
) @TypeOf(src[0]) {
    var sum_r: u32 = 0;
    var sum_g: u32 = 0;
    var sum_b: u32 = 0;
    var sum_a: u32 = 0;
    var total: u32 = 0;

    const kernel = [_][3]u8{
        .{ 1, 2, 1 },
        .{ 2, 4, 2 },
        .{ 3, 6, 3 },
        .{ 2, 4, 2 },
        .{ 1, 2, 1 },
    };

    inline for (kernel, 0..) |row, ky| {
        inline for (row, 0..) |weight, kx| {
            const sample_x = offsetClamped3(x, width, kx);
            const sample_y = offsetClamped5(y, height, ky);
            const px = src[sample_y * width + sample_x];
            const w: u32 = weight;
            sum_r += @as(u32, px.r) * w;
            sum_g += @as(u32, px.g) * w;
            sum_b += @as(u32, px.b) * w;
            sum_a += @as(u32, px.a) * w;
            total += w;
        }
    }

    var out = src[0];
    out.r = @intCast(sum_r / total);
    out.g = @intCast(sum_g / total);
    out.b = @intCast(sum_b / total);
    out.a = @intCast(sum_a / total);
    return out;
}

fn offsetClamped3(base: usize, limit: usize, kernel_index: usize) usize {
    const offset: isize = @as(isize, @intCast(kernel_index)) - 1;
    const sample: isize = @as(isize, @intCast(base)) + offset;
    if (sample < 0) return 0;
    if (sample >= @as(isize, @intCast(limit))) return limit - 1;
    return @intCast(sample);
}

fn offsetClamped5(base: usize, limit: usize, kernel_index: usize) usize {
    const offset: isize = @as(isize, @intCast(kernel_index)) - 2;
    const sample: isize = @as(isize, @intCast(base)) + offset;
    if (sample < 0) return 0;
    if (sample >= @as(isize, @intCast(limit))) return limit - 1;
    return @intCast(sample);
}

fn combineMatrixChannel(orig: u8, blur: u8) u8 {
    const boosted: u32 = (@as(u32, orig) * 1 + @as(u32, blur) * 7) / 4;
    return @intCast(@min(boosted, std.math.maxInt(u8)));
}

/// Add rectangles around contiguous hyperlinks in the render state.
///
/// Note: this currently doesn't take into account unique hyperlink IDs
/// because the render state doesn't contain this. This will be added
/// later.
fn highlightHyperlinks(
    self: *Overlay,
    alloc: Allocator,
    state: *const terminal.RenderState,
) void {
    const border_color = Color.hyperlink.rectBorder();
    const fill_color = Color.hyperlink.rectFill();

    const row_slice = state.row_data.slice();
    const row_raw = row_slice.items(.raw);
    const row_cells = row_slice.items(.cells);
    for (row_raw, row_cells, 0..) |row, cells, y| {
        if (!row.hyperlink) continue;

        const cells_slice = cells.slice();
        const raw_cells = cells_slice.items(.raw);

        var x: usize = 0;
        while (x < raw_cells.len) {
            // Skip cells without hyperlinks
            if (!raw_cells[x].hyperlink) {
                x += 1;
                continue;
            }

            // Found start of a hyperlink run
            const start_x = x;

            // Find end of contiguous hyperlink cells
            while (x < raw_cells.len and raw_cells[x].hyperlink) x += 1;
            const end_x = x;

            self.highlightGridRect(
                alloc,
                start_x,
                y,
                end_x - start_x,
                1,
                border_color,
                fill_color,
            ) catch |err| {
                std.log.warn("Error drawing hyperlink border: {}", .{err});
            };
        }
    }
}

fn highlightSemanticPrompts(
    self: *Overlay,
    alloc: Allocator,
    state: *const terminal.RenderState,
) void {
    const row_slice = state.row_data.slice();
    const row_raw = row_slice.items(.raw);
    const row_cells = row_slice.items(.cells);

    // Highlight the row-level semantic prompt bars. The prompts are easy
    // because they're part of the row metadata.
    {
        const prompt_border = Color.semantic_prompt.rectBorder();
        const prompt_fill = Color.semantic_prompt.rectFill();

        var y: usize = 0;
        while (y < row_raw.len) {
            // If its not a semantic prompt row, skip it.
            if (row_raw[y].semantic_prompt == .none) {
                y += 1;
                continue;
            }

            // Find the full length of the semantic prompt row by connecting
            // all continuations.
            const start_y = y;
            y += 1;
            while (y < row_raw.len and
                row_raw[y].semantic_prompt == .prompt_continuation)
            {
                y += 1;
            }
            const end_y = y; // Exclusive

            const bar_width = @min(@as(usize, 5), self.cell_size.width);
            self.highlightPixelRect(
                alloc,
                0,
                start_y,
                bar_width,
                end_y - start_y,
                prompt_border,
                prompt_fill,
            ) catch |err| {
                log.warn("Error drawing semantic prompt bar: {}", .{err});
            };
        }
    }

    // Highlight contiguous semantic cells within rows.
    for (row_cells, 0..) |cells, y| {
        const cells_slice = cells.slice();
        const raw_cells = cells_slice.items(.raw);

        var x: usize = 0;
        while (x < raw_cells.len) {
            const cell = raw_cells[x];
            const content = cell.semantic_content;
            const start_x = x;

            // We skip output because its just the rest of the non-prompt
            // parts and it makes the overlay too noisy.
            if (cell.semantic_content == .output) {
                x += 1;
                continue;
            }

            // Find the end of this content.
            x += 1;
            while (x < raw_cells.len) {
                const next = raw_cells[x];
                if (next.semantic_content != content) break;
                x += 1;
            }

            const color: Color = switch (content) {
                .prompt => .semantic_prompt,
                .input => .semantic_input,
                .output => unreachable,
            };

            self.highlightGridRect(
                alloc,
                start_x,
                y,
                x - start_x,
                1,
                color.rectBorder(),
                color.rectFill(),
            ) catch |err| {
                log.warn("Error drawing semantic content highlight: {}", .{err});
            };
        }
    }
}

/// Creates a rectangle for highlighting a grid region. x/y/width/height
/// are all in grid cells.
fn highlightGridRect(
    self: *Overlay,
    alloc: Allocator,
    x: usize,
    y: usize,
    width: usize,
    height: usize,
    border_color: z2d.Pixel,
    fill_color: z2d.Pixel,
) !void {
    // All math below uses checked arithmetic to avoid overflows. The
    // inputs aren't trusted and the path this is in isn't hot enough
    // to wrarrant unsafe optimizations.

    // Calculate our width/height in pixels.
    const px_width = std.math.cast(i32, try std.math.mul(
        usize,
        width,
        self.cell_size.width,
    )) orelse return error.Overflow;
    const px_height = std.math.cast(i32, try std.math.mul(
        usize,
        height,
        self.cell_size.height,
    )) orelse return error.Overflow;

    // Calculate pixel coordinates
    const start_x: f64 = @floatFromInt(std.math.cast(i32, try std.math.mul(
        usize,
        x,
        self.cell_size.width,
    )) orelse return error.Overflow);
    const start_y: f64 = @floatFromInt(std.math.cast(i32, try std.math.mul(
        usize,
        y,
        self.cell_size.height,
    )) orelse return error.Overflow);
    const end_x: f64 = start_x + @as(f64, @floatFromInt(px_width));
    const end_y: f64 = start_y + @as(f64, @floatFromInt(px_height));

    // Grab our context to draw
    var ctx: z2d.Context = .init(alloc, &self.surface);
    defer ctx.deinit();

    // Don't need AA because we use sharp edges
    ctx.setAntiAliasingMode(.none);
    // Can use hairline since we have 1px borders
    ctx.setHairline(true);

    // Draw rectangle path
    try ctx.moveTo(start_x, start_y);
    try ctx.lineTo(end_x, start_y);
    try ctx.lineTo(end_x, end_y);
    try ctx.lineTo(start_x, end_y);
    try ctx.closePath();

    // Fill
    ctx.setSourceToPixel(fill_color);
    try ctx.fill();

    // Border
    ctx.setLineWidth(1);
    ctx.setSourceToPixel(border_color);
    try ctx.stroke();
}

/// Creates a rectangle for highlighting a region. x/y are grid cells and
/// width/height are pixels.
fn highlightPixelRect(
    self: *Overlay,
    alloc: Allocator,
    x: usize,
    y: usize,
    width_px: usize,
    height: usize,
    border_color: z2d.Pixel,
    fill_color: z2d.Pixel,
) !void {
    const px_width = std.math.cast(i32, width_px) orelse return error.Overflow;
    const px_height = std.math.cast(i32, try std.math.mul(
        usize,
        height,
        self.cell_size.height,
    )) orelse return error.Overflow;

    const start_x: f64 = @floatFromInt(std.math.cast(i32, try std.math.mul(
        usize,
        x,
        self.cell_size.width,
    )) orelse return error.Overflow);
    const start_y: f64 = @floatFromInt(std.math.cast(i32, try std.math.mul(
        usize,
        y,
        self.cell_size.height,
    )) orelse return error.Overflow);
    const end_x: f64 = start_x + @as(f64, @floatFromInt(px_width));
    const end_y: f64 = start_y + @as(f64, @floatFromInt(px_height));

    var ctx: z2d.Context = .init(alloc, &self.surface);
    defer ctx.deinit();

    ctx.setAntiAliasingMode(.none);
    ctx.setHairline(true);

    try ctx.moveTo(start_x, start_y);
    try ctx.lineTo(end_x, start_y);
    try ctx.lineTo(end_x, end_y);
    try ctx.lineTo(start_x, end_y);
    try ctx.closePath();

    ctx.setSourceToPixel(fill_color);
    try ctx.fill();

    ctx.setLineWidth(1);
    ctx.setSourceToPixel(border_color);
    try ctx.stroke();
}
