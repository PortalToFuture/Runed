const std = @import("std");

const Parser = @import("../../osc.zig").Parser;
const OSCCommand = @import("../../osc.zig").Command;
const matrix9180 = @import("../../matrix9180.zig");

const log = std.log.scoped(.osc_matrix9180);

pub const Command = matrix9180.Command;

pub fn parse(parser: *Parser, _: ?u8) ?*OSCCommand {
    const cap = if (parser.capture) |*c| c else {
        parser.state = .invalid;
        return null;
    };

    cap.writer.writeByte(0) catch {
        parser.state = .invalid;
        return null;
    };

    const raw = cap.trailing();
    const data = raw[0 .. raw.len - 1];
    if (data.len == 0) {
        parser.state = .invalid;
        return null;
    }

    parser.command = .{
        .matrix9180 = parseCommand(data) orelse {
            parser.state = .invalid;
            return null;
        },
    };

    return &parser.command;
}

fn parseCommand(data: []const u8) ?Command {
    if (std.mem.eql(u8, data, "FRAME_END")) return .initFrameEnd();

    const first_sep = std.mem.indexOfScalar(u8, data, ';') orelse {
        log.warn("matrix9180 command missing separator data={s}", .{data});
        return null;
    };

    const op = data[0..first_sep];
    const rest = data[first_sep + 1 ..];

    if (std.mem.eql(u8, op, "LAYER_START")) {
        return .initLayerStart(
            readRequiredInt(rest, "id") orelse return null,
            readRequiredInt(rest, "z") orelse return null,
            readOptionalAlpha(rest, "alpha"),
        );
    }

    if (std.mem.eql(u8, op, "OFFSET")) {
        return .initOffset(
            readRequiredInt(rest, "id") orelse return null,
            readRequiredInt(rest, "x") orelse return null,
            readRequiredInt(rest, "y") orelse return null,
        );
    }

    if (std.mem.eql(u8, op, "DATA")) {
        const second_sep = std.mem.indexOfScalar(u8, rest, ';') orelse return null;
        const id_part = rest[0..second_sep];
        const payload = rest[second_sep + 1 ..];
        return .initData(
            readRequiredInt(id_part, "id") orelse return null,
            payload,
        );
    }

    if (std.mem.eql(u8, op, "LAYER_END")) {
        return .initLayerEnd(readRequiredInt(rest, "id") orelse return null);
    }

    log.warn("unknown matrix9180 op={s}", .{op});
    return null;
}

fn readRequiredInt(raw: []const u8, key: []const u8) ?i32 {
    var it = std.mem.splitScalar(u8, raw, ';');
    while (it.next()) |field| {
        const eql = std.mem.indexOfScalar(u8, field, '=') orelse continue;
        if (!std.mem.eql(u8, field[0..eql], key)) continue;
        return std.fmt.parseInt(i32, field[eql + 1 ..], 10) catch null;
    }

    return null;
}

fn readOptionalAlpha(raw: []const u8, key: []const u8) u8 {
    var it = std.mem.splitScalar(u8, raw, ';');
    while (it.next()) |field| {
        const eql = std.mem.indexOfScalar(u8, field, '=') orelse continue;
        if (!std.mem.eql(u8, field[0..eql], key)) continue;
        const value = std.fmt.parseInt(u16, field[eql + 1 ..], 10) catch return 255;
        return @intCast(@min(value, 255));
    }

    return 255;
}

test "OSC 9180: layer start" {
    const testing = std.testing;
    var p: Parser = .init(testing.allocator);
    defer p.deinit();

    for ("9180;LAYER_START;id=1;z=10;alpha=192") |ch| p.next(ch);

    const cmd = p.end(0x07).?.*;
    try testing.expect(cmd == .matrix9180);
    try testing.expectEqual(.layer_start, cmd.matrix9180.tag);
    try testing.expectEqual(@as(i32, 1), cmd.matrix9180.id);
    try testing.expectEqual(@as(i32, 10), cmd.matrix9180.z_index);
    try testing.expectEqual(@as(u8, 192), cmd.matrix9180.alpha);
}

test "OSC 9180: data payload" {
    const testing = std.testing;
    var p: Parser = .init(testing.allocator);
    defer p.deinit();

    for ("9180;DATA;id=2;⠁⠃\n⠉") |ch| p.next(ch);

    const cmd = p.end(0x07).?.*;
    try testing.expect(cmd == .matrix9180);
    try testing.expectEqual(.data, cmd.matrix9180.tag);
    try testing.expectEqual(@as(i32, 2), cmd.matrix9180.id);
    try testing.expectEqualStrings("⠁⠃\n⠉", cmd.matrix9180.payload());
}

test "OSC 9180: frame end" {
    const testing = std.testing;
    var p: Parser = .init(testing.allocator);
    defer p.deinit();

    for ("9180;FRAME_END") |ch| p.next(ch);

    const cmd = p.end(0x07).?.*;
    try testing.expect(cmd == .matrix9180);
    try testing.expectEqual(.frame_end, cmd.matrix9180.tag);
}
