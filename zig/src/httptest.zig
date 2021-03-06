const olin = @import("./olin/olin.zig");
pub const os = olin;
pub const panic = os.panic;
const log = olin.log;
const resource = olin.resource;

const assert = @import("std").debug.assert;
const mem = @import("std").mem;
const fmt = @import("std").fmt;
const heap = @import("std").heap;
const Headers = @import("std").http.Headers;

const userAgent = "Olin+Zig@master";

pub fn main() anyerror!void {
    log.info("making request to https://xena.greedo.xeserv.us/files/hello_olin.txt");

    try doRequest(heap.page_allocator);
}

fn doRequest(alloc: *mem.Allocator) !void {
    const fout = try resource.open("https://xena.greedo.xeserv.us/files/hello_olin.txt");
    var buf: []u8 = undefined;
    buf = try alloc.alloc(u8, 256);
    defer alloc.free(buf);
    var h = Headers.init(alloc);
    defer h.deinit();
    try h.append("User-Agent", userAgent, null);
    try h.append("Host", "xena.greedo.xeserv.us", null);

    var res = try fmt.bufPrint(buf[0..], "GET /files/hello_olin.txt HTTP/1.1\n{}\n\n", .{ h });
    const n = try fout.write(res);
    log.info(res);

    try fout.flush();

    var resp: []u8 = undefined;
    resp = try alloc.alloc(u8, 2048);
    defer alloc.free(resp);

    const nresp = try fout.read(resp);
    log.info(resp[0..nresp]);
}
