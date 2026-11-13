import zlib, struct, sys
sys.path.insert(0,'shots')
from px import load

def save(path,w,h,buf):
    raw=b''.join(b'\x00'+bytes(buf[y*w*4:(y+1)*w*4]) for y in range(h))
    def chunk(t,d):
        c=t+d
        return struct.pack('>I',len(d))+c+struct.pack('>I',zlib.crc32(c)&0xffffffff)
    ihdr=struct.pack('>IIBBBBB',w,h,8,6,0,0,0)
    open(path,'wb').write(b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',ihdr)+chunk(b'IDAT',zlib.compress(raw,6))+chunk(b'IEND',b''))

def main(src,dst,x0,y0,x1,y1,scale=1):
    w,h,ch,buf=load(src)
    cw,chh=(x1-x0)*scale,(y1-y0)*scale
    out=bytearray(cw*chh*4)
    for y in range(chh):
        sy=y0+y//scale
        for x in range(cw):
            sx=x0+x//scale
            o=(sy*w+sx)*ch
            d=(y*cw+x)*4
            out[d]=buf[o]; out[d+1]=buf[o+1]; out[d+2]=buf[o+2]; out[d+3]=255
    save(dst,cw,chh,out)
    print('saved',dst,cw,chh)

if __name__=='__main__':
    a=sys.argv
    main(a[1],a[2],int(a[3]),int(a[4]),int(a[5]),int(a[6]),int(a[7]) if len(a)>7 else 1)
