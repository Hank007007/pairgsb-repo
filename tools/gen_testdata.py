"""Regenerates the captured upstream responses under testdata/.

Each file isolates one property of the intake rules in docs/dns-intake.md.
Run with:  python3 -I tools/gen_testdata.py
"""
import os
import struct

OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "testdata")


def name(s):
    if s in (".", ""):
        return b"\x00"
    out = b""
    for lab in s.rstrip(".").split("."):
        out += bytes([len(lab)]) + lab.encode("ascii")
    return out + b"\x00"


def hdr(id=0x1234, flags=0x8180, qd=0, an=0, ns=0, ar=0):
    return struct.pack(">HHHHHH", id, flags, qd, an, ns, ar)


def q(namebytes, t=1, c=1):
    return namebytes + struct.pack(">HH", t, c)


def rr(owner, t, c, ttl, rdata):
    return owner + struct.pack(">HHIH", t, c, ttl, len(rdata)) + rdata


def write(fn, data):
    with open(os.path.join(OUT, fn), "wb") as f:
        f.write(data)


QN = name("www.example.com")
PTR_Q = b"\xc0\x0c"

normal = hdr(qd=1, an=1) + q(QN) + rr(PTR_Q, 1, 1, 300, bytes([93, 184, 216, 34]))

# --- ordinary responses -------------------------------------------------
write("normal-a.bin", normal)
write("nxdomain.bin", hdr(flags=0x8183, qd=1) + q(name("nope.example.com")))

# a CNAME chain that stays inside the question name's bailiwick
zone_q = name("example.com")
chain_in = hdr(qd=1, an=2) + q(zone_q)
chain_in += rr(b"\xc0\x0c", 5, 1, 300, name("cdn.example.com"))
chain_in += rr(name("cdn.example.com"), 1, 1, 300, bytes([5, 6, 7, 8]))
write("cname-chain-in.bin", chain_in)

# --- compression pointer abuse -----------------------------------------
# the name at offset 12 is a pointer back to offset 12 itself
write("ptr-self-loop.bin", hdr(qd=1) + b"\xc0\x0c" + struct.pack(">HH", 1, 1))

# two pointers that point at each other
two_loop = bytearray()
two_loop += hdr(qd=1)
two_loop += b"\xc0\x0e\xc0\x0c"
two_loop += struct.pack(">HH", 1, 1)
write("ptr-mutual-loop.bin", bytes(two_loop))

fwd = bytearray()
fwd += hdr(qd=1)
fwd += b"\xc0\x12" + struct.pack(">HH", 1, 1)
fwd += name("example.com")
write("ptr-forward.bin", bytes(fwd))

mid = bytearray()
mid += hdr(qd=1)
mid += b"\x04ab3c"
mid += b"\xc0\x0f"
mid += struct.pack(">HH", 1, 1)
mid += b"a" * 44
mid += b"\x00"
write("ptr-mid-label.bin", bytes(mid))

# 100 records whose owner names form a chain: owner[i] is one label plus a
# pointer to owner[i-1]. Resolving the last owner follows 99 pointers, and the
# whole message needs 4950 pointer follows in total.
N = 100
bomb = bytearray()
bomb += hdr(qd=0, an=N)
owner_off = []
off = 12
for i in range(N):
    owner_off.append(off)
    if i == 0:
        bomb += b"\x01a\x00"
        off += 3
    else:
        bomb += b"\x01a" + struct.pack(">H", 0xC000 | owner_off[i - 1])
        off += 4
    bomb += struct.pack(">HHIH", 1, 1, 300, 4) + bytes([10, 0, 0, i])
    off += 14
write("ptr-bomb.bin", bytes(bomb))

# --- name and label limits ---------------------------------------------
longn = b""
for _ in range(5):
    longn += b"\x3f" + b"a" * 63
longn += b"\x00"
write("name-too-long.bin", hdr(qd=1) + q(longn))
write("label-too-long.bin", hdr(qd=1) + q(b"\x7f" + b"a" * 127 + b"\x00"))
write("reserved-label.bin", hdr(qd=1) + q(b"\x40" + b"b" * 64 + b"\x00"))
write("bad-ascii-label.bin", hdr(qd=1) + q(b"\x04_dns" + b"\x02\xc3\xa9" + b"\x00"))
write("label-with-dot.bin", hdr(qd=1) + q(b"\x04bad." + name("example.com")))

# --- RDATA handling -----------------------------------------------------
# The CNAME RDATA claims two bytes but the encoded name is five bytes long;
# a second answer follows immediately.
badrd = bytearray()
badrd += hdr(qd=1, an=2)
badrd += q(QN)
badrd += PTR_Q + struct.pack(">HHIH", 5, 1, 300, 2)
badrd += b"\x03abc\x00"
badrd += rr(PTR_Q, 1, 1, 300, bytes([1, 2, 3, 4]))
write("rdata-bad-rdlength.bin", bytes(badrd))

# CNAME rdata is a pointer to offset 12, which is the question name. The
# question is cdn.example.com while the record's owner is www.example.com, so
# a correct decode yields the target cdn.example.com. Another answer follows so
# that bytes exist past the two-byte RDATA.
rptr = hdr(qd=1, an=2) + q(name("cdn.example.com"))
rptr += rr(name("www.example.com"), 5, 1, 300, b"\xc0\x0c")
rptr += rr(b"\xc0\x0c", 1, 1, 300, bytes([9, 8, 7, 6]))
write("rdata-name-pointer.bin", rptr)

# compressed name inside an unrecognised type's RDATA. The pointer is written
# so that it resolves against the RDATA slice itself; read against the message
# header it does not denote a label boundary.
unk = hdr(qd=1, an=1) + q(QN) + rr(PTR_Q, 65280, 1, 300, b"\xc0\x02\x01a\x00")
write("rdata-compressed-unknown.bin", unk)

# three declared answers; the middle record's owner pointer runs past the end
partial = bytearray()
partial += hdr(qd=1, an=3)
partial += q(QN)
partial += rr(PTR_Q, 1, 1, 300, bytes([10, 0, 0, 1]))
partial += b"\xc0\xff" + struct.pack(">HHIH", 1, 1, 300, 4) + bytes([10, 0, 0, 2])
partial += rr(PTR_Q, 1, 1, 300, bytes([10, 0, 0, 3]))
write("partial-records.bin", bytes(partial))

# --- EDNS ---------------------------------------------------------------
badopt = hdr(qd=1, an=1, ar=1) + q(QN) + rr(PTR_Q, 1, 1, 300, bytes([1, 2, 3, 4]))
badopt += rr(b"\x00", 41, 256, 0, b"")
write("opt-bad-udpsize.bin", badopt)

opt = hdr(qd=1, ar=1) + q(QN)
opt += rr(b"\x00", 41, 1232, 0,
          struct.pack(">HH", 65001, 2) + b"hi" +
          struct.pack(">HH", 3, 4) + b"nsid")
write("opt-unknown-option.bin", opt)

# --- header, truncation, trailing bytes --------------------------------
cm = hdr(qd=1, an=2) + q(QN) + rr(PTR_Q, 1, 1, 300, bytes([5, 6, 7, 8]))
write("count-mismatch.bin", cm)

qd2 = hdr(qd=2) + q(name("a.example.com")) + q(name("b.example.com"))
write("qdcount-2.bin", qd2)

write("tc-set.bin",
      hdr(flags=0x8380, qd=1, an=1) + q(QN) + rr(PTR_Q, 1, 1, 300, bytes([9, 9, 9, 9])))
write("trailing-bytes.bin", normal + b"\xde\xad\xbe")

# --- cache screening ----------------------------------------------------
# chain whose middle hop leaves the bailiwick of the question name
chain = bytearray()
chain += hdr(qd=1, an=3)
chain += q(QN)
chain += rr(PTR_Q, 5, 1, 300, name("cdn.example.com"))
chain += rr(name("cdn.example.com"), 5, 1, 300, name("evil-example.com"))
chain += rr(name("evil-example.com"), 1, 1, 300, bytes([6, 6, 6, 6]))
write("cname-chain-out.bin", bytes(chain))

# chain head echoes the question exactly; the next hop changes case
casehop = hdr(qd=1, an=2) + q(name("WwW.Example.COM"))
casehop += rr(name("WwW.Example.COM"), 5, 1, 300, name("cdn.example.com"))
casehop += rr(name("CDN.Example.COM"), 1, 1, 300, bytes([4, 4, 4, 4]))
write("cname-case-hop.bin", casehop)

# answer owner is a different registrable domain that merely ends with the
# same characters
sfx = hdr(qd=1, an=1) + q(name("example.com"))
sfx += rr(name("evil-example.com"), 1, 1, 300, bytes([7, 7, 7, 7]))
write("suffix-confusion.bin", sfx)

# question uses 0x20 mixed case, answer owner is the lower-case spelling
case = hdr(qd=1, an=1) + q(name("WwW.Example.COM"))
case += rr(name("www.example.com"), 1, 1, 300, bytes([8, 8, 8, 8]))
write("case-mismatch.bin", case)

# --- TTL ----------------------------------------------------------------
write("ttl-huge.bin",
      hdr(qd=1, an=1) + q(QN) + rr(PTR_Q, 1, 1, 0xFFFFFFFF, bytes([1, 1, 1, 1])))
write("ttl-wrap.bin",
      hdr(qd=1, an=1) + q(QN) + rr(PTR_Q, 1, 1, 0x80000000, bytes([1, 1, 1, 2])))
write("ttl-zero.bin",
      hdr(qd=1, an=1) + q(QN) + rr(PTR_Q, 1, 1, 0, bytes([2, 2, 2, 2])))

# --- two responses back to back ----------------------------------------
write("two-responses.bin", normal + normal)

print("wrote", len([f for f in os.listdir(OUT) if f.endswith(".bin")]), "files")
