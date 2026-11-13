import sys
exec(open('shots/px.py').read().split('def main')[0])

path = sys.argv[1]
w, h, ch, buf = load(path)
cols, rows = 8, 6
print('size', w, h)
print('mean luminance grid (0-255):')
hdr = '        ' + ''.join(f'{i:>7d}' for i in range(cols))
print(hdr)
for cy in range(rows):
    y0 = cy * h // rows
    y1 = (cy + 1) * h // rows
    line = f'row{cy:<4d}'
    for cx in range(cols):
        x0 = cx * w // cols
        x1 = (cx + 1) * w // cols
        tot = 0.0
        n = 0
        for j in range(y0, y1, 2):
            base = j * w * ch
            for i in range(x0, x1, 2):
                o = base + i * ch
                tot += 0.2126 * buf[o] + 0.7152 * buf[o+1] + 0.0722 * buf[o+2]
                n += 1
        line += f'{tot/n:7.1f}'
    print(line)
# 极暗像素占比（死黑区域）
dark = 0
tot = 0
for j in range(0, h, 3):
    base = j * w * ch
    for i in range(0, w, 3):
        o = base + i * ch
        lum = 0.2126 * buf[o] + 0.7152 * buf[o+1] + 0.0722 * buf[o+2]
        if lum < 8:
            dark += 1
        tot += 1
print(f'dark(<8) ratio {dark/tot*100:.1f}%')
