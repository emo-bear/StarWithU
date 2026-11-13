import zlib, struct, sys

def load(path):
    d = open(path, 'rb').read()
    assert d[:8] == b'\x89PNG\r\n\x1a\n'
    pos = 8
    w = h = bitd = ct = None
    idat = b''
    while pos < len(d):
        ln = struct.unpack('>I', d[pos:pos+4])[0]
        typ = d[pos+4:pos+8]
        body = d[pos+8:pos+8+ln]
        if typ == b'IHDR':
            w, h, bitd, ct = struct.unpack('>IIBB', body[:10])
        elif typ == b'IDAT':
            idat += body
        pos += 12 + ln
    raw = zlib.decompress(idat)
    ch = {0: 1, 2: 3, 4: 2, 6: 4}[ct]
    stride = w * ch
    out = bytearray(h * stride)
    prev = bytearray(stride)
    p = 0
    for y in range(h):
        f = raw[p]; p += 1
        line = bytearray(raw[p:p+stride]); p += stride
        if f == 1:
            for i in range(ch, stride):
                line[i] = (line[i] + line[i-ch]) & 255
        elif f == 2:
            for i in range(stride):
                line[i] = (line[i] + prev[i]) & 255
        elif f == 3:
            for i in range(stride):
                a = line[i-ch] if i >= ch else 0
                line[i] = (line[i] + ((a + prev[i]) >> 1)) & 255
        elif f == 4:
            for i in range(stride):
                a = line[i-ch] if i >= ch else 0
                b = prev[i]
                c = prev[i-ch] if i >= ch else 0
                pp = a + b - c
                pa, pb, pc = abs(pp-a), abs(pp-b), abs(pp-c)
                pr = a if (pa <= pb and pa <= pc) else (b if pb <= pc else c)
                line[i] = (line[i] + pr) & 255
        out[y*stride:(y+1)*stride] = line
        prev = line
    return w, h, ch, bytes(out)

def main():
    path = sys.argv[1]
    w, h, ch, buf = load(path)
    print('size', w, h, 'ch', ch)
    pts = [
        ('corner_TR', 1030, 25), ('TR_upper', 980, 80), ('TR_mid', 900, 150),
        ('top_mid', 530, 18), ('top_left', 40, 25), ('TL_mid', 150, 120),
        ('left_void', 150, 470), ('center', 530, 360), ('bot_mid', 530, 700),
        ('BR', 1020, 690), ('right_mid', 1035, 400), ('above_core', 530, 120),
    ]
    for name, x, y in pts:
        tot = [0, 0, 0]; n = 0
        for i in range(max(0, x-14), min(w, x+14)):
            for j in range(max(0, y-14), min(h, y+14)):
                o = (j*w + i)*ch
                tot[0] += buf[o]; tot[1] += buf[o+1]; tot[2] += buf[o+2]; n += 1
        r, g, b = [round(v/n, 1) for v in tot]
        print(f'{name:12s} ({x},{y})  rgb=({r},{g},{b})  lum={round(0.2126*r+0.7152*g+0.0722*b,1)}')

if __name__ == '__main__':
    main()
