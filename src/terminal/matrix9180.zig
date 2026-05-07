const std = @import("std");

const Allocator = std.mem.Allocator;

pub const Command = extern struct {
    tag: Tag,
    id: i32 = 0,
    z_index: i32 = 0,
    x_offset: i32 = 0,
    y_offset: i32 = 0,
    payload_ptr: [*]const u8 = undefined,
    payload_len: usize = 0,

    pub const Tag = enum(c_int) {
        layer_start,
        offset,
        data,
        layer_end,
        frame_end,
    };

    pub fn initLayerStart(id: i32, z_index: i32) Command {
        return .{
            .tag = .layer_start,
            .id = id,
            .z_index = z_index,
        };
    }

    pub fn initOffset(id: i32, x_offset: i32, y_offset: i32) Command {
        return .{
            .tag = .offset,
            .id = id,
            .x_offset = x_offset,
            .y_offset = y_offset,
        };
    }

    pub fn initData(id: i32, bytes: []const u8) Command {
        return .{
            .tag = .data,
            .id = id,
            .payload_ptr = bytes.ptr,
            .payload_len = bytes.len,
        };
    }

    pub fn initLayerEnd(id: i32) Command {
        return .{
            .tag = .layer_end,
            .id = id,
        };
    }

    pub fn initFrameEnd() Command {
        return .{ .tag = .frame_end };
    }

    pub fn payload(self: Command) []const u8 {
        return self.payload_ptr[0..self.payload_len];
    }
};

pub const Layer = struct {
    id: i32,
    z_index: i32 = 0,
    x_offset: i32 = 0,
    y_offset: i32 = 0,
    data: []u8 = &.{},
    closed: bool = false,

    pub fn deinit(self: *Layer, alloc: Allocator) void {
        if (self.data.len > 0) alloc.free(self.data);
        self.* = undefined;
    }

    pub fn clone(self: Layer, alloc: Allocator) !Layer {
        return .{
            .id = self.id,
            .z_index = self.z_index,
            .x_offset = self.x_offset,
            .y_offset = self.y_offset,
            .data = try alloc.dupe(u8, self.data),
            .closed = self.closed,
        };
    }

    pub fn rowCount(self: Layer) usize {
        return countRows(self.data);
    }

    pub fn maxCols(self: Layer) usize {
        return countMaxCols(self.data);
    }
};

pub const Frame = struct {
    layers: std.ArrayListUnmanaged(Layer) = .{},

    pub const empty: Frame = .{};

    pub fn deinit(self: *Frame, alloc: Allocator) void {
        self.clearRetainingCapacity(alloc);
        self.layers.deinit(alloc);
        self.* = .empty;
    }

    pub fn clearRetainingCapacity(self: *Frame, alloc: Allocator) void {
        for (self.layers.items) |*layer| layer.deinit(alloc);
        self.layers.clearRetainingCapacity();
    }

    pub fn clone(self: Frame, alloc: Allocator) !Frame {
        var result: Frame = .empty;
        errdefer result.deinit(alloc);

        try result.layers.ensureTotalCapacity(alloc, self.layers.items.len);
        for (self.layers.items) |layer| {
            result.layers.appendAssumeCapacity(try layer.clone(alloc));
        }

        return result;
    }

    pub fn findLayer(self: *Frame, id: i32) ?*Layer {
        for (self.layers.items) |*layer| {
            if (layer.id == id) return layer;
        }

        return null;
    }

    pub fn ensureLayer(self: *Frame, alloc: Allocator, id: i32) !*Layer {
        if (self.findLayer(id)) |layer| return layer;

        try self.layers.append(alloc, .{ .id = id });
        return &self.layers.items[self.layers.items.len - 1];
    }
};

pub fn countRows(data: []const u8) usize {
    if (data.len == 0) return 0;

    var rows: usize = 1;
    for (data) |ch| {
        if (ch == '\n') rows += 1;
    }

    return rows;
}

pub fn countMaxCols(data: []const u8) usize {
    var rows = std.mem.splitScalar(u8, data, '\n');
    var max: usize = 0;
    while (rows.next()) |row| {
        const cols = std.unicode.utf8CountCodepoints(row) catch row.len;
        max = @max(max, cols);
    }

    return max;
}

pub fn applyCommand(
    frame: *Frame,
    alloc: Allocator,
    cmd: Command,
) !bool {
    switch (cmd.tag) {
        .layer_start => {
            const layer = try frame.ensureLayer(alloc, cmd.id);
            if (layer.data.len > 0) {
                alloc.free(layer.data);
                layer.data = &.{};
            }

            layer.z_index = cmd.z_index;
            layer.x_offset = 0;
            layer.y_offset = 0;
            layer.closed = false;
            return false;
        },

        .offset => {
            const layer = try frame.ensureLayer(alloc, cmd.id);
            layer.x_offset = cmd.x_offset;
            layer.y_offset = cmd.y_offset;
            return false;
        },

        .data => {
            const layer = try frame.ensureLayer(alloc, cmd.id);
            if (layer.data.len > 0) alloc.free(layer.data);
            layer.data = try alloc.dupe(u8, cmd.payload());
            return false;
        },

        .layer_end => {
            const layer = try frame.ensureLayer(alloc, cmd.id);
            layer.closed = true;
            return false;
        },

        .frame_end => return true,
    }
}

test "matrix9180 frame lifecycle" {
    const testing = std.testing;

    var frame: Frame = .empty;
    defer frame.deinit(testing.allocator);

    try testing.expect(!(try applyCommand(&frame, testing.allocator, .initLayerStart(1, 10))));
    try testing.expect(!(try applyCommand(&frame, testing.allocator, .initOffset(1, -1, 2))));
    try testing.expect(!(try applyCommand(&frame, testing.allocator, .initData(1, "abc"))));
    try testing.expect(!(try applyCommand(&frame, testing.allocator, .initLayerEnd(1))));
    try testing.expect(try applyCommand(&frame, testing.allocator, .initFrameEnd()));

    try testing.expectEqual(@as(usize, 1), frame.layers.items.len);
    const layer = frame.layers.items[0];
    try testing.expectEqual(@as(i32, 1), layer.id);
    try testing.expectEqual(@as(i32, 10), layer.z_index);
    try testing.expectEqual(@as(i32, -1), layer.x_offset);
    try testing.expectEqual(@as(i32, 2), layer.y_offset);
    try testing.expectEqualStrings("abc", layer.data);
    try testing.expect(layer.closed);
}

test "matrix9180 frame clone" {
    const testing = std.testing;

    var frame: Frame = .empty;
    defer frame.deinit(testing.allocator);

    _ = try applyCommand(&frame, testing.allocator, .initLayerStart(2, 11));
    _ = try applyCommand(&frame, testing.allocator, .initData(2, "payload"));

    var clone = try frame.clone(testing.allocator);
    defer clone.deinit(testing.allocator);

    try testing.expectEqual(@as(usize, 1), clone.layers.items.len);
    try testing.expectEqualStrings("payload", clone.layers.items[0].data);
}
