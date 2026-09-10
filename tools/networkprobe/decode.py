"""Decode client QUIC v1 Initials from the loopback capture (no TLS secrets)."""
import base64
import hashlib
import hmac
import json
import sys
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

def varint(b, pos):
    n = 1 << (b[pos] >> 6)
    return int.from_bytes(b[pos:pos+n], 'big') & ((1 << (8*n-2))-1), pos+n

def expand(secret, label, length):
    label = b'tls13 '+label
    info = length.to_bytes(2,'big')+bytes([len(label)])+label+b'\0'
    return hmac.new(secret, info+b'\1', hashlib.sha256).digest()[:length]

def initial(packet, original_dcid=None):
    b = bytearray(packet)
    assert b[1:5] == b'\0\0\0\1'
    n = b[5]; dcid=bytes(b[6:6+n]); pos=6+n
    pos += 1+b[pos]
    n,pos = varint(b,pos); pos+=n
    length,pos = varint(b,pos)
    secret = hmac.new(bytes.fromhex('38762cf7f55934b34d179ae6a4c80cadccbb7f0a'), original_dcid or dcid, hashlib.sha256).digest()
    secret = expand(secret,b'client in',32)
    hp = expand(secret,b'quic hp',16)
    mask = Cipher(algorithms.AES(hp), modes.ECB()).encryptor().update(bytes(b[pos+4:pos+20]))
    b[0] ^= mask[0]&15
    pnlen = (b[0]&3)+1
    for i in range(pnlen): b[pos+i] ^= mask[i+1]
    pn = int.from_bytes(b[pos:pos+pnlen],'big')
    iv = int.from_bytes(expand(secret,b'quic iv',12),'big')^pn
    payload = AESGCM(expand(secret,b'quic key',16)).decrypt(iv.to_bytes(12,'big'),bytes(b[pos+pnlen:pos+length]),bytes(b[:pos+pnlen]))
    p=0; chunks=[]
    while p<len(payload):
        typ,p = varint(payload,p)
        if typ in (0,1): continue
        if typ==6:
            off,p=varint(payload,p); n,p=varint(payload,p)
            chunks.append((off,payload[p:p+n]));p+=n
        elif typ in (2,3):
            _,p=varint(payload,p);_,p=varint(payload,p);count,p=varint(payload,p);_,p=varint(payload,p)
            for _ in range(count*2+(3 if typ==3 else 0)): _,p=varint(payload,p)
        else: break
    return dcid.hex(),chunks

def hello(b):
    p=4+2+32; sid=b[p];p+=1+sid
    n=int.from_bytes(b[p:p+2],'big');p+=2
    ciphers=[b[i:i+2].hex() for i in range(p,p+n,2)];p+=n
    p+=1+b[p];n=int.from_bytes(b[p:p+2],'big');p+=2;end=p+n
    extensions=[]
    while p<end:
        typ=int.from_bytes(b[p:p+2],'big');n=int.from_bytes(b[p+2:p+4],'big');p+=4
        extensions.append({'id':typ,'data':b[p:p+n].hex()});p+=n
    return {'sessionIDLength':sid,'ciphers':ciphers,'extensions':extensions}

def decode(path):
    streams={}
    peers={}
    for line in open(path):
        row=json.loads(line)
        if 'initial' not in row: continue
        packet=base64.b64decode(row['initial'])
        original=peers.setdefault(row['peer'],packet[6:6+packet[5]])
        _,chunks=initial(packet,original)
        key=original.hex()
        stream=streams.setdefault(key,{})
        for off,data in chunks:
            for i,v in enumerate(data): stream[off+i]=v
    results=[]
    for key,data in streams.items():
        if not all(i in data for i in range(4)): continue
        n=int.from_bytes(bytes(data[i] for i in range(1,4)),'big')+4
        if not all(i in data for i in range(n)): continue
        results.append(hello(bytes(data[i] for i in range(n))))
    return results

if __name__=='__main__':
    print(json.dumps(decode(sys.argv[1]),indent=2))
