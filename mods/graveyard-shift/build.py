#!/usr/bin/env python3
"""Build Graveyard Shift E1M1: original art, MUS music, and classic DOOM nodes.

Requires Python 3, Pillow and ZDBSP for the curved geometry. No Boom or engine changes.
PNG sources are retained in assets/textures; edit them and use --keep-art to rebuild.
"""
from __future__ import annotations
import argparse
from collections import Counter, defaultdict, deque
from dataclasses import dataclass, replace
import json
import math
from pathlib import Path
import random
import struct
import shutil
import subprocess
import tempfile
import os
from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
UNIT = 64
CREAM = (237, 211, 157)
RED = (142, 39, 51)
TEAL = (27, 89, 86)
GOLD = (211, 157, 66)
INK = (22, 24, 39)
FONT = '/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf'


def name(s):
    assert len(s) <= 8, s
    return s.encode('ascii').ljust(8, b'\0')


def read_wad(path):
    raw = Path(path).read_bytes()
    sig, count, offset = struct.unpack_from('<4sii', raw)
    assert sig in (b'IWAD', b'PWAD')
    result = {}
    for i in range(count):
        pos, size, key = struct.unpack_from('<ii8s', raw, offset + 16*i)
        result[key.rstrip(b'\0').decode('ascii')] = raw[pos:pos+size]
    return result


def write_wad(lumps, path):
    data = bytearray(12)
    directory = bytearray()
    for key, raw in lumps:
        directory.extend(struct.pack('<ii8s', len(data), len(raw), name(key)))
        data.extend(raw)
    struct.pack_into('<4sii', data, 0, b'PWAD', len(lumps), len(data))
    path.write_bytes(data + directory)


def font(size):
    return ImageFont.truetype(FONT, size)


def centered(d, text, y, size, color, width, inset=6):
    f = font(size)
    while d.textbbox((0, 0), text, font=f)[2] > width-2*inset:
        size -= 1
        f = font(size)
    box = d.textbbox((0, 0), text, font=f)
    d.text(((width-(box[2]-box[0]))//2, y-box[1]), text, font=f, fill=color)


def bulbs(d, w, h):
    d.rectangle((1, 1, w-2, h-2), outline=GOLD, width=3)
    for x in range(8, w-6, 12):
        for y in [7, h-8]:
            d.ellipse((x-2, y-2, x+2, y+2), fill=CREAM)
    for y in range(19, h-15, 12):
        for x in [7, w-8]:
            d.ellipse((x-2, y-2, x+2, y+2), fill=CREAM)


def sign(lines, colors=(INK, GOLD), size=(128, 128)):
    im = Image.new('RGBA', size, colors[0])
    d = ImageDraw.Draw(im)
    bulbs(d, *size)
    line_h = (size[1]-32)//len(lines)
    for i, line in enumerate(lines):
        centered(d, line, 18+i*line_h+(line_h-18)//2, min(20, line_h-6), colors[1], size[0], inset=19)
    return im


def make_art(keep=False):
    art = {}
    rng = random.Random(1989)
    # Palette-conscious, intentionally pixel-sized architectural materials.
    for key, base, accent in [('GSTRIPE', CREAM, RED), ('GTEAL', TEAL, GOLD), ('GWOOD', (72, 45, 42), (112, 77, 55)), ('GBRICK', (48, 51, 65), (70, 74, 84)), ('GMETAL', (47, 58, 60), (71, 85, 84))]:
        im = Image.new('RGBA', (128, 128), base)
        d = ImageDraw.Draw(im)
        if key == 'GSTRIPE':
            for x in range(0, 128, 32):
                d.rectangle((x, 0, x+15, 127), fill=accent)
            d.rectangle((0, 0, 127, 12), fill=GOLD)
            d.rectangle((0, 115, 127, 127), fill=GOLD)
            d.line((0, 14, 127, 14), fill=INK, width=2)
        elif key == 'GBRICK':
            for y in range(0, 128, 16):
                d.line((0, y, 127, y), fill=INK, width=2)
                for x in range(-32 if y % 32 else 0, 128, 64):
                    d.line((x, y, x, y+16), fill=INK, width=2)
        elif key == 'GWOOD':
            for x in range(0, 128, 16):
                d.line((x, 0, x, 127), fill=INK, width=2)
                for y in range(8, 128, 24):
                    d.point((x+3, y), fill=accent)
                    d.line((x+8, y-4, x+8, y+7), fill=accent)
            for y in [8, 112]:
                d.rectangle((0, y, 127, y+5), fill=(43, 36, 35))
        else:
            for x in [0, 63, 127]:
                d.line((x, 0, x, 127), fill=accent, width=2)
            for y in [8, 112]:
                d.rectangle((0, y, 127, y+4), fill=accent)
            for x in [8, 56, 72, 120]:
                for y in [18, 104]:
                    d.ellipse((x-1, y-1, x+1, y+1), fill=GOLD)
        # Restrained deterministic wear, with crisp signs left unweathered.
        for _ in range(150):
            x, y = rng.randrange(128), rng.randrange(128)
            r, g, b, a = im.getpixel((x, y))
            delta = rng.choice([-12, -8, 8])
            d.point((x, y), fill=tuple(max(0, min(255, c+delta)) for c in (r, g, b))+(a,))
        art[key] = im
    art['GDOOR'] = sign(['STAFF', 'ACCESS'], (TEAL, CREAM))
    art['GBLUE'] = sign(['ADMIT ONE', 'BLUE PASS', 'NO REFUNDS'], ((23, 44, 89), CREAM), size=(256,128))
    art['GSHUTTR'] = Image.new('RGBA',(128,128),(37,49,52))
    sd = ImageDraw.Draw(art['GSHUTTR'])
    for y in range(0,128,8):
        sd.line((0,y,127,y),fill=(17,25,31),width=2)
        sd.line((0,y+2,127,y+2),fill=(63,75,73))
    for x in [4,123]:
        sd.line((x,0,x,127),fill=GOLD,width=2)
    art['GPOWER'] = sign(['ADMISSION', 'POWER', 'PRESS USE'], (TEAL, GOLD))
    art['GON'] = sign(['POWER ON', 'THANK YOU', 'NOW WORK'], (TEAL, CREAM))
    art['GWIN'] = sign(['WINNER!', 'CLAIM PRIZE', 'NO REFUNDS'], (TEAL, CREAM))
    art['GEXIT'] = sign(['TO THE', 'MIDWAY', 'PRESS USE'], (RED, CREAM))
    art['GSAFETY'] = sign(['YOUR SAFETY', 'IS OUR', '4TH PRIORITY'])
    art['GSICK'] = sign(['POSSESSION', 'IS NOT', 'SICK LEAVE'])
    art['GSNACK'] = sign(['EMPLOYEE', 'BENEFITS', 'TRY THE WALL'])
    art['GPRIZE'] = sign(['GRAND PRIZE', 'ARMOR!', 'NO REFUNDS'], (RED, CREAM))
    art['GSECURE'] = sign(['SECURITY', 'TAKE ONE', 'SHOTGUN'], (TEAL, CREAM))
    art['GOPEN'] = sign(['WELCOME', 'WE ARE', 'STILL OPEN'], (RED, CREAM))
    art['GTARGET'] = sign(['SHOOT TO WIN', '', 'ONE SHOT!'], (INK, GOLD))
    d = ImageDraw.Draw(art['GTARGET'])
    for radius, color in [(23, CREAM), (17, RED), (11, CREAM), (5, RED)]:
        d.ellipse((64-radius, 65-radius, 64+radius, 65+radius), fill=color)
    # Masked wrought-iron fence; transparent holes are encoded as patch posts.
    im = Image.new('RGBA', (128, 128))
    d = ImageDraw.Draw(im)
    for x in range(8, 128, 16):
        d.line((x, 12, x, 127), fill=(56, 73, 73), width=3)
        d.polygon([(x-4, 12), (x, 3), (x+4, 12)], fill=GOLD)
    for y in [35, 100]:
        d.rectangle((0, y, 127, y+3), fill=(73, 91, 87))
    art['GFENCE'] = im
    # Overhead entry marquee. The lower half is open to walk beneath.
    im = Image.new('RGBA', (256, 128))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, 255, 93), radius=10, fill=RED, outline=GOLD, width=4)
    for x in range(10, 256, 12):
        d.ellipse((x-2, 5, x+2, 9), fill=CREAM)
        d.ellipse((x-2, 84, x+2, 88), fill=CREAM)
    centered(d, 'GRAVEYARD SHIFT', 18, 21, CREAM, 256, inset=19)
    centered(d, 'ADMISSION IS MANDATORY', 53, 14, GOLD, 256, inset=19)
    art['GARCH'] = im
    # Built-in vanilla animation names: no ANIMATED lump or Boom dependency.
    # Three phases walk warm bulbs around the entrance marquee perimeter.
    perimeter = [(x,7) for x in range(10,250,12)]
    perimeter += [(248,y) for y in range(19,83,12)]
    perimeter += [(x,86) for x in range(238,0,-12)]
    perimeter += [(7,y) for y in range(74,10,-12)]
    for phase in range(3):
        frame = art['GARCH'].copy()
        fd = ImageDraw.Draw(frame)
        for i,(x,y) in enumerate(perimeter):
            color = (255,245,190) if i % 3 == phase else (103,68,35)
            fd.ellipse((x-2,y-2,x+2,y+2),fill=color)
        art[f'GSTFONT{phase+1}'] = frame
        frame = art['GSTRIPE'].copy()
        fd = ImageDraw.Draw(frame)
        for i,x in enumerate(range(8,128,16)):
            for y in [6,121]:
                color = (255,245,190) if i % 3 == phase else (103,68,35)
                fd.ellipse((x-3,y-3,x+3,y+3),fill=INK)
                fd.ellipse((x-2,y-2,x+2,y+2),fill=color)
        art[f'SLADRIP{phase+1}'] = frame
    # Original night sky with a crescent moon, stars and a distant wheel.
    im = Image.new('RGBA', (256, 128))
    d = ImageDraw.Draw(im)
    for y in range(128):
        t = y/127
        d.line((0, y, 255, y), fill=(int(16+30*t), int(18+8*t), int(37+25*t)))
    for _ in range(80):
        x, y = rng.randrange(256), rng.randrange(85)
        d.point((x, y), fill=(157, 159, 175))
    d.ellipse((155, 15, 187, 47), fill=CREAM)
    d.ellipse((165, 10, 192, 40), fill=(24, 21, 44))
    d.ellipse((27, 51, 91, 115), outline=(99, 65, 83), width=2)
    for a in range(0, 360, 45):
        x, y = 59+32*math.cos(math.radians(a)), 83+32*math.sin(math.radians(a))
        d.line((59, 83, x, y), fill=(99, 65, 83))
        d.rectangle((x-2, y-1, x+2, y+3), fill=(161, 104, 81))
    d.polygon([(45, 127), (59, 83), (74, 127)], outline=(99, 65, 83))
    for x in range(0, 256, 16):
        h = rng.randrange(9, 25)
        d.rectangle((x, 128-h, x+15, 127), fill=(16, 18, 29))
        d.polygon([(x-2, 128-h), (x+8, 117-h), (x+18, 128-h)], fill=(16, 18, 29))
    art['SKY1'] = im
    # Flats use the same small, purposeful material family.
    for key in ['GASPHALT', 'GPAVING', 'GCHECK', 'GPLANK', 'GGRATE', 'GCARPET', 'GWATER', 'GROOF']:
        im = Image.new('RGBA', (64, 64), INK)
        d = ImageDraw.Draw(im)
        if key == 'GCHECK':
            for x in range(0, 64, 16):
                for y in range(0, 64, 16):
                    d.rectangle((x, y, x+15, y+15), fill=CREAM if (x+y)//16 % 2 else RED)
        elif key in ['GPAVING', 'GASPHALT']:
            base = (66, 67, 72) if key == 'GPAVING' else (41, 43, 49)
            d.rectangle((0, 0, 63, 63), fill=base)
            for _ in range(350):
                x, y = rng.randrange(64), rng.randrange(64)
                v = rng.randrange(-10, 11)
                d.point((x, y), fill=tuple(c+v for c in base))
            if key == 'GPAVING':
                for y in range(0, 64, 16):
                    d.line((0, y, 63, y), fill=(43, 44, 49))
                    for x in range((y//16 % 2)*16, 64, 32):
                        d.line((x, y, x, y+16), fill=(43, 44, 49))
        elif key == 'GPLANK':
            d.rectangle((0, 0, 63, 63), fill=(89, 57, 43))
            for y in range(0, 64, 16):
                d.line((0, y, 63, y), fill=INK, width=2)
                for x in range(4, 64, 13):
                    d.line((x, y+5, x+7, y+5), fill=(116, 77, 52))
        elif key == 'GGRATE':
            for x in range(0, 64, 8):
                d.rectangle((x, 0, x+2, 63), fill=(77, 93, 88))
            for y in range(0, 64, 8):
                d.rectangle((0, y, 63, y+1), fill=(54, 66, 67))
        elif key == 'GCARPET':
            d.rectangle((0, 0, 63, 63), fill=RED)
            d.rectangle((2, 2, 61, 61), outline=GOLD, width=2)
            d.polygon([(32, 12), (37, 27), (52, 32), (37, 37), (32, 52), (27, 37), (12, 32), (27, 27)], fill=(177, 107, 51))
        elif key == 'GWATER':
            d.rectangle((0, 0, 63, 63), fill=(0, 0, 131))
            for y in range(0, 64, 8):
                for x in range(0, 64, 2):
                    d.point((x, (y+int(2*math.sin(x/7))) % 64), fill=(55, 55, 255))
        else:
            d.rectangle((0, 0, 63, 63), fill=(45, 55, 58))
            d.rectangle((2, 2, 61, 61), outline=(88, 104, 93), width=2)
        art[key] = im
    # Four seamless ripple phases use DOOM's built-in FWATER flat cycle.
    for phase in range(4):
        im = Image.new('RGBA',(64,64))
        for y in range(64):
            for x in range(64):
                wave = math.sin(2*math.pi*((x+y)/32-phase/4))
                cross = math.cos(2*math.pi*((x-y)/16+phase/4))
                glow = int(13*wave+9*cross)
                # Saturated blues survive the stock PLAYPAL quantization;
                # muted cyan/teal would collapse to gray in DOOM's palette.
                color = (8,12,150+glow)
                if wave+cross > 1.50:
                    color = (55,55,255)
                im.putpixel((x,y),color+(255,))
        art[f'FWATER{phase+1}'] = im
    # A small coordinated architectural kit: quieter facades make the red
    # marquee and round carousel read as focal points rather than wallpaper.
    im=Image.new('RGBA',(128,128),(176,147,104));d=ImageDraw.Draw(im)
    for x in range(0,128,16):d.line((x,16,x,104),fill=(141,113,78))
    d.rectangle((0,0,127,15),fill=(66,38,30));d.rectangle((0,5,127,8),fill=GOLD)
    d.rectangle((0,105,127,127),fill=(59,39,35))
    for y in [105,117]:
        d.line((0,y,127,y),fill=INK)
        for x in range(0 if y==105 else 16,128,32):d.line((x,y,x,y+12),fill=INK)
    art['GFACADE']=im
    im=im.copy();d=ImageDraw.Draw(im)
    d.rectangle((17,29,110,92),fill=(52,31,29),outline=GOLD,width=3)
    d.rectangle((23,35,104,86),fill=(34,26,35))
    d.rectangle((29,42,98,80),fill=(124,73,43))
    d.line((64,33,64,89),fill=GOLD,width=3);d.line((20,60,108,60),fill=GOLD,width=3)
    art['GWINDOW']=im
    im=art['GFACADE'].copy();d=ImageDraw.Draw(im)
    d.rectangle((0,0,127,23),fill=RED)
    centered(d,'TICKETS',5,14,CREAM,128,inset=19)
    d.rectangle((14,29,113,83),fill=INK,outline=GOLD,width=3)
    d.rectangle((21,36,106,77),fill=(58,34,35))
    d.line((64,33,64,80),fill=(142,100,53),width=3)
    d.rectangle((7,84,120,95),fill=(87,45,31),outline=GOLD,width=2)
    d.rectangle((85,64,98,74),fill=CREAM);d.line((88,67,95,67),fill=RED)
    art['GBOOTH']=im
    im=Image.new('RGBA',(128,128),RED);d=ImageDraw.Draw(im)
    for x in range(-16,144,32):
        d.polygon([(x,14),(x+16,14),(x+26,52),(x+10,52)],fill=CREAM)
        d.pieslice((x,26,x+32,58),0,180,fill=GOLD)
    d.rectangle((0,0,127,13),fill=(87,49,31))
    for x in range(8,128,16):d.ellipse((x-3,3,x+3,9),fill=CREAM)
    d.line((0,64,127,64),fill=GOLD,width=3)
    art['GVALANCE']=im
    im=Image.new('RGBA',(128,128),(88,57,34));d=ImageDraw.Draw(im)
    for x in range(0,128,16):
        d.rectangle((x+2,0,x+6,127),fill=GOLD)
        d.line((x+8,0,x+8,127),fill=(49,30,29),width=2)
    for y in [3,119]:d.rectangle((0,y,127,y+5),fill=CREAM)
    art['GPOLE']=im
    im=art['GMETAL'].copy();d=ImageDraw.Draw(im)
    d.rectangle((12,20,115,87),fill=INK,outline=GOLD,width=2)
    for x in range(21,111,9):d.line((x,28,x,77),fill=(103,115,106),width=3)
    for x in range(12,116,16):
        d.polygon([(x,99),(x+8,99),(x,111),(x-8,111)],fill=GOLD)
    d.rectangle((16,36,40,65),fill=(47,35,33),outline=CREAM)
    d.line((27,60,35,44),fill=RED,width=3)
    art['GGEN']=im
    im=art['GMETAL'].copy();d=ImageDraw.Draw(im)
    d.line((0,0,127,0),fill=GOLD,width=2)
    d.line((0,15,127,15),fill=(102,111,108))
    art['GSTEP']=im
    im=art['GWOOD'].copy();d=ImageDraw.Draw(im)
    d.rectangle((5,5,122,122),outline=(139,91,49),width=8)
    d.line((11,12,116,115),fill=(139,91,49),width=9)
    for x in [11,116]:
        for y in [11,116]:d.ellipse((x-2,y-2,x+2,y+2),fill=GOLD)
    art['GCRATE']=im
    im=Image.new('RGBA',(64,128));d=ImageDraw.Draw(im)
    d.line((32,0,32,127),fill=GOLD,width=4)
    d.line((30,0,30,127),fill=CREAM)
    d.ellipse((7,58,53,85),fill=CREAM,outline=(116,74,42),width=2)
    d.polygon([(39,65),(40,42),(48,34),(52,45),(61,51),(58,60),(48,61),(47,72)],fill=CREAM)
    d.polygon([(40,42),(44,36),(43,62),(38,68)],fill=RED)
    d.polygon([(8,63),(2,55),(3,78),(11,72)],fill=RED)
    for x in [14,22,41,48]:
        d.line((x,79,x-3,103),fill=CREAM,width=5)
        d.line((x-6,104,x+1,104),fill=GOLD,width=3)
    d.rectangle((25,58,39,69),fill=RED,outline=GOLD,width=2)
    d.ellipse((48,46,50,48),fill=INK)
    art['GHORSE']=im
    fringe=art['GVALANCE'].copy()
    fringe.paste((0,0,0,0),(0,32,128,128))
    art['GFRINGE']=fringe
    art['GRIDE']=sign(['GRINNING','GALLOP','ADMIT ONE'],(RED,CREAM))
    # Official customer-care policy, readable from the carousel's north path.
    # The top 32 pixels become the plain baseboard on this 160-unit wall
    # when classic DOOM repeats the 128-high texture: never repeat the text.
    im=Image.new('RGBA',(256,128),(52,31,29));d=ImageDraw.Draw(im)
    for x in range(0,256,32):d.line((x,0,x,127),fill=(38,24,25),width=2)
    d.rectangle((1,33,254,126),fill=INK,outline=GOLD,width=2)
    for x in range(9,250,12):
        for y in [39,120]:d.ellipse((x-2,y-2,x+2,y+2),fill=CREAM)
    for y in range(51,112,12):
        for x in [8,247]:d.ellipse((x-2,y-2,x+2,y+2),fill=CREAM)
    centered(d,'UAC CUSTOMER CARE',45,10,GOLD,256,inset=24)
    for text,y in [('ALL SCREAMS ARE',62),('CONSIDERED POSITIVE',81),('CUSTOMER FEEDBACK.',100)]:
        centered(d,text,y,16,CREAM,256,inset=24)
    art['GSCREAM']=im

    for key,color in [('GREDTOP',(124,30,40)),('GCREAM',(205,177,124)),('GVELVET',(107,27,38))]:
        im=Image.new('RGBA',(64,64),color);d=ImageDraw.Draw(im)
        for y in range(0,64,4):d.line((0,y,63,y),fill=tuple(max(0,c-5) for c in color))
        art[key]=im
    im=Image.new('RGBA',(64,64),(33,35,40));d=ImageDraw.Draw(im)
    d.rectangle((6,15,57,48),fill=(67,59,43),outline=GOLD,width=2)
    d.rectangle((10,21,53,42),fill=CREAM)
    d.line((31,20,31,44),fill=(89,64,39),width=2)
    art['GLAMP']=im
    folder = HERE/'assets/textures'
    folder.mkdir(parents=True, exist_ok=True)
    for key, im in art.items():
        path = folder/f'{key}.png'
        if keep and path.exists():
            art[key] = Image.open(path).convert('RGBA')
        else:
            im.save(path)
    atlas = Image.new('RGB', (768, math.ceil(len(art)/6)*158), INK)
    d = ImageDraw.Draw(atlas)
    for i, (key, im) in enumerate(art.items()):
        x, y = (i % 6)*128, (i//6)*158
        show = im.copy()
        show.thumbnail((128, 128), Image.Resampling.NEAREST)
        atlas.paste(show, (x, y), show)
        d.text((x+4, y+132), key, font=font(11), fill=CREAM)
    atlas.save(HERE/'previews/texture-atlas.png')
    previews = []
    for step in range(12):
        panel = Image.new('RGB',(620,280),INK)
        pd = ImageDraw.Draw(panel)
        pd.text((18,14),'CHASING LIGHTS + RIPPLING WATER',font=font(17),fill=CREAM)
        a = art[f'GSTFONT{step%3+1}'].resize((256,128),Image.Resampling.NEAREST)
        panel.paste(a,(18,59),a)
        a = art[f'SLADRIP{step%3+1}'].resize((128,128),Image.Resampling.NEAREST)
        panel.paste(a,(298,59),a)
        a = art[f'FWATER{step%4+1}'].resize((128,128),Image.Resampling.NEAREST)
        panel.paste(a,(452,59),a)
        for x,label in [(18,'Entrance marquee'),(298,'Kiosk / carousel'),(452,'Fountain water')]:
            pd.text((x,212),label,font=font(12),fill=CREAM)
        pd.text((18,249),'Classic DOOM animations / 8 tics per frame / no Boom required',font=font(12),fill=GOLD)
        previews.append(panel)
    previews[0].save(HERE/'previews/animated-textures.gif',save_all=True,append_images=previews[1:],duration=229,loop=0)
    return art


def indexed(im, palette):
    pal = Image.new('P', (1, 1))
    pal.putpalette(palette)
    return im.convert('RGB').quantize(palette=pal, dither=Image.Dither.NONE)


def patch(im, palette):
    w, h = im.size
    assert h <= 254, 'Classic patch posts must fit the single-byte height'
    pixels = indexed(im, palette).tobytes()
    alpha = im.getchannel('A').tobytes()
    header = bytearray(struct.pack('<hhhh', w, h, 0, 0) + bytes(4*w))
    for x in range(w):
        struct.pack_into('<I', header, 8+4*x, len(header))
        y = 0
        while y < h:
            if not alpha[y*w+x]:
                y += 1
                continue
            start = y
            while y < h and alpha[y*w+x]:
                y += 1
            header.extend(bytes([start, y-start, 0]))
            header.extend(pixels[row*w+x] for row in range(start, y))
            header.append(0)
        header.append(255)
    return bytes(header)


def art_lumps(art, base):
    palette = base['PLAYPAL'][:768]
    # Preserve the selected IWAD's patch-name indices and texture metadata.
    # This also keeps the normal next map usable while the episode is in progress.
    count, = struct.unpack_from('<I', base['PNAMES'])
    pnames = [base['PNAMES'][4+i*8:12+i*8] for i in range(count)]
    originals = {}
    for table in ['TEXTURE1','TEXTURE2']:
        raw = base.get(table, struct.pack('<I',0))
        num, = struct.unpack_from('<I',raw)
        for i in range(num):
            pos, = struct.unpack_from('<I',raw,4+4*i)
            patches, = struct.unpack_from('<h',raw,pos+20)
            key = raw[pos:pos+8].rstrip(b'\0').decode('ascii')
            originals[key] = raw[pos:pos+22+10*patches]
    textures = []
    texture_defs = {}
    lumps = []
    flats = {'GASPHALT', 'GPAVING', 'GCHECK', 'GPLANK', 'GGRATE', 'GCARPET', 'GWATER', 'GROOF', 'FWATER1', 'FWATER2', 'FWATER3', 'FWATER4', 'GREDTOP', 'GCREAM', 'GVELVET', 'GLAMP'}
    for key, im in art.items():
        if key in flats:
            continue
        pname = 'P'+key if len(key) < 8 else key
        pi = len(pnames)
        pnames.append(name(pname))
        w, h = im.size
        texture_defs[key] = (w,h,pi)
        textures.append(struct.pack('<8sIhhIh', name(key), 0, w, h, 0, 1) + struct.pack('<hhhhh', 0, 0, pi, 0, 0))
        lumps.append((pname, patch(im, palette)))
    # Reinsert each classic animation range as one contiguous sequence. The
    # original IWAD may already contain these slots in a different position.
    groups = [('GSTFONT1','GSTFONT2','GSTFONT3'), ('SLADRIP1','SLADRIP2','SLADRIP3')]
    for group in groups:
        for key in group:
            originals.pop(key,None)
    # Vanilla initializes this fixed switch list even when a map doesn't use
    # every pair. Supply aliases made from our art, plus a dummy texture zero.
    aliases = {}
    suffixes = 'BRCOM BRN1 BRN2 BRNGN BROWN COMM COMP DIRT EXIT GRAY GRAY1 METAL PIPE SLAD STARG STON1 STON2 STONE STRTN BLUE CMT GARG GSTON HOT LION SATYR SKIN VINE WOOD PANEL ROCK MET2 WDMET BRIK MOD1 ZIM STON6 TEK MARB SKULL'.split()
    for suffix in suffixes:
        for n in [1,2]:
            key = f'SW{n}{suffix}'
            if key not in originals:
                aliases[key] = 'GMETAL'
    aliases.update(SW1COMM='GPOWER', SW2COMM='GON', SW1EXIT='GEXIT', SW2EXIT='GEXIT', SW1GARG='GTARGET', SW2GARG='GWIN')
    for alias,source in aliases.items():
        w,h,pi = texture_defs[source]
        raw = struct.pack('<8sIhhIh', name(alias), 0, w, h, 0, 1)+struct.pack('<hhhhh',0,0,pi,0,0)
        if alias == 'AASHITTY':
            textures.insert(0,raw)
        else:
            textures.append(raw)
    # Override selected signs/sky in place; vanilla resolves the first matching
    # texture name, so duplicate definitions in TEXTURE2 would not be portable.
    for raw in textures:
        key = raw[:8].rstrip(b'\0').decode('ascii')
        originals[key] = raw
    for group in groups:
        frames = [(key,originals.pop(key)) for key in group]
        for key,raw in frames:
            originals[key] = raw
    textures = list(originals.values())
    offsets = []
    pos = 4+4*len(textures)
    for raw in textures:
        offsets.append(pos)
        pos += len(raw)
    lumps += [('PNAMES', struct.pack('<I', len(pnames))+b''.join(pnames)),
              ('TEXTURE1', struct.pack('<I', len(textures))+b''.join(struct.pack('<I', o) for o in offsets)+b''.join(textures)), ('TEXTURE2', struct.pack('<I',0)), ('FF_START', b'')]
    for key in sorted(flats):
        assert art[key].size == (64, 64), key
        lumps.append((key, indexed(art[key], palette).tobytes()))
    lumps.append(('F_END', b''))
    # Native intermission/menu graphics; no base-game pixels are copied.
    im = Image.new('RGBA', (300, 18))
    centered(ImageDraw.Draw(im), 'ADMISSION IS MANDATORY', 1, 16, CREAM, 300)
    lumps.append(('WILV00', patch(im, palette)))
    im = Image.new('RGBA', (218, 22))
    centered(ImageDraw.Draw(im), 'GRAVEYARD SHIFT', 1, 21, GOLD, 218)
    lumps.append(('M_EPI1', patch(im, palette)))
    im = art['SKY1'].resize((320, 200), Image.Resampling.NEAREST).convert('RGBA')
    d = ImageDraw.Draw(im)
    d.rectangle((13, 43, 306, 153), fill=INK, outline=GOLD, width=3)
    centered(d, 'GRAVEYARD', 54, 36, CREAM, 320)
    centered(d, 'SHIFT', 94, 35, RED, 320)
    centered(d, 'A UAC FAMILY ATTRACTION', 137, 12, GOLD, 320)
    centered(d, 'MAP 1 / ADMISSION IS MANDATORY', 175, 12, CREAM, 320)
    im.save(HERE/'assets/title.png')
    lumps.append(('TITLEPIC', patch(im, palette)))
    return lumps


@dataclass(frozen=True)
class Style:
    label: str
    floor: int = 0
    ceil: int = 256
    flat: str = 'GPAVING'
    roof: str = 'F_SKY1'
    light: int = 176
    wall: str = 'GSTRIPE'
    special: int = 0
    tag: int = 0
    door: bool = False


class Level:
    def __init__(self):
        self.cells = {}
        self.carved = set()
        self.extra_polygons = []
        self.extra_panels = []
        self.things = []
        self.wall_overrides = {}
        self.edge_overrides = {}
        self.landmarks = {}
        self.manifest = {}

    def rect(self, x0, y0, x1, y1, style):
        for x in range(x0, x1):
            for y in range(y0, y1):
                self.cells[x, y] = style

    def erase(self, x0, y0, x1, y1):
        for x in range(x0, x1):
            for y in range(y0, y1):
                self.cells.pop((x, y), None)

    def thing(self, x, y, typ, flags=7, angle=0):
        # Fractional cell coordinates are centers or authored intra-cell points.
        self.things.append((round(x*UNIT), round(y*UNIT), angle, typ, flags))

    def sign(self, a, b, texture, special=0, tag=0):
        # Unit-length authored wall segments; north walls point east, east walls south.
        ax, ay = a
        bx, by = b
        n = max(abs(bx-ax), abs(by-ay))
        for i in range(n):
            p = (ax+(bx-ax)*i//n, ay+(by-ay)*i//n)
            q = (ax+(bx-ax)*(i+1)//n, ay+(by-ay)*(i+1)//n)
            self.wall_overrides[frozenset((p, q))] = (texture, i*UNIT, special, tag)

    def edge(self, a, b, **kw):
        self.edge_overrides[frozenset((a, b))] = kw

    def round_area(self, cx, cy, bounds, bands, outside, sides=16):
        """Concentric convex wedges with a thin spindle and stepped canopy.

        bounds is a grid-aligned rectangular canvas; bands are outside-in
        (radius in world units, Style). ZDBSP builds normal classic nodes.
        """
        x0,y0,x1,y1=bounds
        assert cx==(x0+x1)/2 and cy==(y0+y1)/2
        def points(radius):
            return [(round((cx*UNIT+radius*math.cos(-i*2*math.pi/sides))/8)*8/UNIT,
                     round((cy*UNIT+radius*math.sin(-i*2*math.pi/sides))/8)*8/UNIT) for i in range(sides)]
        square=[]
        for i in range(sides):
            a=-i*2*math.pi/sides;co,si=math.cos(a),math.sin(a)
            distance=min((x1-cx)/abs(co) if abs(co)>1e-8 else 1e9,
                         (y1-cy)/abs(si) if abs(si)>1e-8 else 1e9)
            square.append((round((cx+distance*co)*UNIT)/UNIT,round((cy+distance*si)*UNIT)/UNIT))
        outer=square
        for radius,style in bands:
            inner=points(radius) if radius else [(cx,cy)]*sides
            for i in range(sides):
                j=(i+1)%sides
                # Include canvas corners between rays; a rectangular canvas
                # cannot be represented by the ray endpoints alone.
                corners=[]
                if outer is square:
                    for corner in [(x0,y0),(x0,y1),(x1,y1),(x1,y0)]:
                        angle=(-math.atan2(corner[1]-cy,corner[0]-cx))%(2*math.pi)
                        position=angle*sides/(2*math.pi)
                        if i+1e-7<position<i+1-1e-7:corners.append((position,corner))
                poly=[outer[i]]+[p for _,p in sorted(corners)]+[outer[j],inner[j],inner[i]]
                unique=[]
                for point in poly:
                    if not unique or point!=unique[-1]:unique.append(point)
                if len(unique)>1 and unique[0]==unique[-1]:unique.pop()
                area=sum(a[0]*b[1]-b[0]*a[1] for a,b in zip(unique,unique[1:]+unique[:1]))
                if abs(area)<1e-6:continue
                actual=style(i) if callable(style) else style
                self.extra_polygons.append((unique,actual))
            outer=inner
        for x in range(x0,x1):
            for y in range(y0,y1):
                self.carved.add((x,y))
                distance=math.hypot((x+.5-cx)*UNIT,(y+.5-cy)*UNIT)
                style=outside
                for radius,candidate in bands:
                    if distance>=radius:
                        style=candidate(0) if callable(candidate) else candidate
                        break
                self.cells[x,y]=style

    def panel(self, a, b, texture, style, blocking=False):
        self.extra_panels.append((a,b,texture,style,blocking))

    def build(self):
        # Clockwise convex cells form input geometry for the classic node builder.
        vertices, vertex_index = [], {}
        sectors, sector_index = [], {}
        sides, lines, edge_index = [], [], {}
        subsectors, segs, cell_subsector = [], [], {}
        def vertex(p):
            if p not in vertex_index:
                vertex_index[p] = len(vertices)
                vertices.append((round(p[0]*UNIT), round(p[1]*UNIT)))
            return vertex_index[p]
        def sector(style):
            if style not in sector_index:
                sector_index[style] = len(sectors)
                sectors.append(style)
            return sector_index[style]
        polygons=[]
        for (x,y),style in sorted(self.cells.items()):
            if (x,y) not in self.carved:
                polygons.append(([(x,y),(x,y+1),(x+1,y+1),(x+1,y)],style,(x,y)))
        polygons.extend((poly,style,None) for poly,style in self.extra_polygons)
        # Node every shared polygon edge at existing vertices, including the
        # intersections between the sixteen-sided canvas and the grid walls.
        points=set(p for poly,_,_ in polygons for p in poly)
        def noded(poly):
            result=[]
            for a,b in zip(poly,poly[1:]+poly[:1]):
                dx,dy=b[0]-a[0],b[1]-a[1];length2=dx*dx+dy*dy
                cuts=[]
                for p in points:
                    cross=(p[0]-a[0])*dy-(p[1]-a[1])*dx
                    t=((p[0]-a[0])*dx+(p[1]-a[1])*dy)/length2
                    if abs(cross)<1e-7 and 0<=t<1-1e-8:cuts.append((t,p))
                result.extend(p for _,p in sorted(cuts))
            return result
        for poly,style,cell in polygons:
            sec=sector(style);poly=noded(poly)
            if cell is not None:cell_subsector[cell]=len(subsectors)
            start=len(segs)
            for a,b in zip(poly,poly[1:]+poly[:1]):
                mx,my=(a[0]+b[0])/2,(a[1]+b[1])/2
                nx=math.floor(mx-(b[1]-a[1])*.51);ny=math.floor(my+(b[0]-a[0])*.51)
                neighbor=self.cells.get((nx,ny))
                if style.door and neighbor==style:continue
                va,vb=vertex(a),vertex(b);key=frozenset((a,b))
                sd=len(sides);sides.append([0,0,'-','-',style.wall,sec])
                if key in edge_index:
                    li=edge_index[key];line=lines[li]
                    assert line[6]==65535,('overlapping geometry',a,b)
                    line[2]=4;line[6]=sd
                    sides[line[5]][2:5]=[style.wall,style.wall,'-']
                    other_wall=sectors[sides[line[5]][5]].wall
                    sides[sd][2:5]=[other_wall,other_wall,'-'];direction=1
                else:
                    li=len(lines);edge_index[key]=li
                    lines.append([va,vb,1,0,0,sd,65535]);direction=0
                angle=round(math.atan2(b[1]-a[1],b[0]-a[0])*65536/(2*math.pi))&65535
                segs.append([va,vb,angle,li,direction,0])
            assert len(segs)>start
            subsectors.append((len(segs)-start,start))
        for a,b,texture,style,blocking in self.extra_panels:
            sec=sector(style);sd=len(sides)
            sides.extend([[0,0,'-','-',texture,sec],[0,0,'-','-',texture,sec]])
            li=len(lines);key=frozenset((a,b));edge_index[key]=li
            lines.append([vertex(a),vertex(b),4|16|128|(1 if blocking else 0),0,0,sd,sd+1])
        # Apply linedef behavior only after front/back sides exist.
        action_lines = defaultdict(list)
        for key, li in edge_index.items():
            line = lines[li]
            front = sectors[sides[line[5]][5]]
            back = sectors[sides[line[6]][5]] if line[6] != 65535 else None
            if back and (front.door or back.door):
                door = front if front.door else back
                if front.door and not back.door:
                    line[0], line[1], line[5], line[6] = line[1], line[0], line[6], line[5]
                    for seg in segs:
                        if seg[3] == li:
                            seg[4] ^= 1
                    front, back = back, front
                for si in [line[5], line[6]]:
                    sides[si][2] = 'GBLUE' if door.label == 'admission gate' else ('GSTRIPE' if door.label in ('garden door', 'snack door') else ('GDOOR' if door.label == 'security door' else 'GSHUTTR'))
                if door.label == 'admission gate':
                    line[3] = 32
                elif door.label in ('security door', 'garden door', 'snack door'):
                    line[3] = 31
                    if door.label != 'security door':
                        line[2] |= 32  # Automap secret, preserve visible mystery.
                # Remote shutters/ambush closets have no local use action.
                if line[3]:
                    action_lines[door.label].append(li)
            if key in self.wall_overrides:
                # Keep the playable side at the front of the existing wall.
                if front.label.startswith('sign backing '):
                    line[0],line[1],line[5],line[6]=line[1],line[0],line[6],line[5]
                    for seg in segs:
                        if seg[3]==li:seg[4]^=1
                    front,back=back,front
                tex, offset, special, tag = self.wall_overrides[key]
                sd = sides[line[5]]
                sd[0], sd[4] = offset, tex
                if back and back.label.startswith('sign backing '):
                    # One wall plane: background above, poster below. Standard
                    # sidedef offsets place each chunk across the same face.
                    sd[1]=0
                    sd[2:5]=[front.wall,tex,'-']
                    line[2]|=1
                line[3:5] = [special, tag]
                if special:
                    action_lines[tex].append(li)
            if key in self.edge_overrides:
                ov = self.edge_overrides[key]
                line[2] |= ov.get('flags', 0)
                line[3:5] = [ov.get('special', 0), ov.get('tag', 0)]
                for si in [line[5], line[6]]:
                    if si != 65535:
                        if 'mid' in ov:
                            sides[si][4] = ov['mid']
                        offset = ov.get('offset', sides[si][0])
                        if si == line[6] and ov.get('width'):
                            offset = (ov['width']-offset-round(math.dist(*key)*UNIT)) % ov['width']
                        sides[si][0] = offset
                        if 'row_offset' in ov:sides[si][1]=ov['row_offset']
                if line[3]:
                    action_lines[ov.get('label', 'trigger')].append(li)
            # Keep construction-only same-sector lines off the automap.
            if back == front and not line[3] and sides[line[5]][4] == '-':
                line[2] |= 128
            # Align repeats to the world grid instead of restarting every 64 units.
            a, b = vertices[line[0]], vertices[line[1]]
            if key not in self.wall_overrides and key not in self.edge_overrides:
                for si, p, q in [(line[5], a, b), (line[6], b, a)]:
                    if si == 65535:
                        continue
                    sides[si][0] = ((p[0] if q[0] > p[0] else -p[0]) if p[1] == q[1] else (p[1] if q[1] > p[1] else -p[1])) % 128
        # Door lettering spans an entire face. Anchor each oriented face at
        # its own edge instead of using a world-space modulo that cuts signs
        # in half when a doorway begins on an odd 64-unit cell.
        door_faces = defaultdict(list)
        for li,line in enumerate(lines):
            if line[6] == 65535:
                continue
            back = sectors[sides[line[6]][5]]
            if not back.door:
                continue
            a,b = vertices[line[0]],vertices[line[1]]
            dx,dy = b[0]-a[0],b[1]-a[1]
            axis = 0 if dx else 1
            direction = 1 if (dx or dy) > 0 else -1
            constant = a[1-axis]
            key = (sides[line[6]][5],axis,direction,constant)
            door_faces[key].append((li,a[axis]*direction,b[axis]*direction))
        for face in door_faces.values():
            origin = min(min(a,b) for _,a,b in face)
            for li,a,b in face:
                sides[lines[li][5]][0] = a-origin
        # Anchor the lettering independently on each contiguous ticket-booth face.
        booth_faces = defaultdict(list)
        for line in lines:
            a,b = vertices[line[0]],vertices[line[1]]
            for si,p,q in [(line[5],a,b),(line[6],b,a)]:
                if si==65535 or sides[si][3]!='GBOOTH':continue
                axis=0 if p[1]==q[1] else 1
                direction=1 if q[axis]>p[axis] else -1
                booth_faces[(axis,direction,p[1-axis])].append((si,p[axis]*direction,q[axis]*direction))
        for face in booth_faces.values():
            end=None;origin=0
            for si,a,b in sorted(face,key=lambda item:min(item[1:])):
                if end is None or min(a,b)>end:origin=min(a,b)
                sides[si][0]=a-origin
                end=max(a,b)
        # Architectural materials share a vertical world origin across rooms
        # and risers. Posters, doors and ride trim retain their own placement.
        aligned={'GMETAL','GTEAL','GWOOD','GBRICK','GFACADE'}
        for line in lines:
            face=[i for i in [line[5],line[6]] if i!=65535]
            textures={tex for i in face for tex in sides[i][2:5] if tex!='-'}
            if not textures or not textures<=aligned:continue
            line[2]|=8|16
            for i in face:sides[i][1]=-sectors[sides[i][5]].ceil%128
        nodes = []
        def bbox(cells):
            return [(max(y for x,y in cells)+1)*UNIT, min(y for x,y in cells)*UNIT,
                    min(x for x,y in cells)*UNIT, (max(x for x,y in cells)+1)*UNIT]
        def tree(cells):
            if len(cells) == 1:
                return 0x8000 + cell_subsector[cells[0]]
            xs = sorted(set(x for x,y in cells))
            ys = sorted(set(y for x,y in cells))
            axis = 0 if len(xs) >= len(ys) else 1
            values = xs if axis == 0 else ys
            split = values[len(values)//2]
            lower = [c for c in cells if c[axis] < split]
            upper = [c for c in cells if c[axis] >= split]
            assert lower and upper
            # Vertical upward partition: east is right. Horizontal eastward:
            # south is right. Child 0 must always lie on the right side.
            right, left = (upper, lower) if axis == 0 else (lower, upper)
            cr, cl = tree(right), tree(left)
            x,y,dx,dy = (split*UNIT, 0, 0, UNIT) if axis == 0 else (0, split*UNIT, UNIT, 0)
            ni = len(nodes)
            nodes.append((x,y,dx,dy,*bbox(right),*bbox(left),cr,cl))
            return ni
        tree(sorted(cell_subsector))
        # Conservative per-cell lists, including touching boundary lines. No
        # unsigned blockmap offsets exceed the vanilla 16-bit limit.
        ox, oy = min(x for x,y in vertices)-8, min(y for x,y in vertices)-8
        width = (max(x for x,y in vertices)-ox)//128+1
        height = (max(y for x,y in vertices)-oy)//128+1
        blocks = [[] for _ in range(width*height)]
        for li, line in enumerate(lines):
            a, b = vertices[line[0]], vertices[line[1]]
            x0 = max(0, (min(a[0],b[0])-ox-1)//128)
            x1 = min(width-1, (max(a[0],b[0])-ox+1)//128)
            y0 = max(0, (min(a[1],b[1])-oy-1)//128)
            y1 = min(height-1, (max(a[1],b[1])-oy+1)//128)
            for by in range(y0, y1+1):
                for bx in range(x0, x1+1):
                    blocks[by*width+bx].append(li)
        words = [ox,oy,width,height]+[0]*(width*height)
        lists = {}
        for i, block in enumerate(blocks):
            key = tuple(block)
            if key not in lists:
                lists[key] = len(words)
                words += [0]+block+[-1]
            words[4+i] = lists[key]
        assert len(words) < 65536
        pack = lambda fmt, rows: b''.join(struct.pack(fmt,*row) for row in rows)
        lumps = [('E1M1', b''), ('THINGS', pack('<5h', self.things)), ('LINEDEFS', pack('<7H', lines)),
                 ('SIDEDEFS', b''.join(struct.pack('<hh8s8s8sH', x,y,name(t),name(b),name(m),s) for x,y,t,b,m,s in sides)),
                 ('VERTEXES', pack('<2h', vertices)), ('SEGS', pack('<6H', segs)),
                 ('SSECTORS', pack('<2H', subsectors)), ('NODES', pack('<12h2H', nodes)),
                 ('SECTORS', b''.join(struct.pack('<hh8s8shhh', s.floor,s.ceil,name(s.flat),name(s.roof),s.light,s.special,s.tag) for s in sectors)),
                 ('REJECT', bytes((len(sectors)**2+7)//8)),
                 ('BLOCKMAP', b''.join(struct.pack('<H', w & 65535) for w in words))]
        self.manifest = dict(map='E1M1', title='Admission Is Mandatory', format='vanilla Doom',
                             vertices=len(vertices), linedefs=len(lines), sectors=len(sectors),
                             subsectors=len(subsectors), nodes=len(nodes), secrets=sum(s.special == 9 for s in sectors),
                             actions=dict(action_lines), sector_labels={s.label:i for i,s in enumerate(sectors)},
                             sector_tags={str(tag):[i for i,s in enumerate(sectors) if s.tag == tag] for tag in [10,20,30,40]},
                             landmarks=self.landmarks,
                             animations={'entrance_marquee':['GSTFONT1','GSTFONT2','GSTFONT3'],
                                         'chasing_trim':['SLADRIP1','SLADRIP2','SLADRIP3'],
                                         'fountain_water':['FWATER1','FWATER2','FWATER3','FWATER4']})
        monsters = {3004:'zombieman',9:'shotgunner',3001:'imp',3002:'pinky'}
        self.manifest['difficulty'] = {}
        for label, flag in [('easy',1),('hurt_me_plenty',2),('ultra_violence',4)]:
            counts = Counter(monsters[t] for x,y,a,t,f in self.things if t in monsters and f&flag)
            self.manifest['difficulty'][label] = dict(monsters=sum(counts.values()), types=dict(counts))
        return lumps


def make_level():
    l = Level()
    plaza = Style('ticket plaza',ceil=176,light=160)
    arrival = Style('arrival', ceil=128, flat='GASPHALT', wall='GBRICK', light=160)
    office = Style('security office', ceil=128, roof='GROOF', flat='GCHECK', wall='GTEAL', light=160)
    hall = Style('staff corridor', ceil=112, roof='GROOF', flat='GGRATE', wall='GMETAL', light=128)
    yard = Style('service yard', ceil=128, flat='GASPHALT', wall='GBRICK', light=160)
    control = Style('control booth', floor=112, ceil=256, roof='GROOF', flat='GGRATE', wall='GTEAL', light=192)
    balcony = Style('balcony', floor=112, ceil=320, flat='GPLANK', wall='GWOOD', light=192)
    final = Style('welcome courtyard', wall='GFACADE', light=160)
    # Arrival, readable central hub, and the western staff loop.
    l.rect(0,-8,11,0,arrival)
    l.rect(5,0,9,3,replace(plaza,label='entry arch',ceil=256))
    l.rect(-2,3,18,21,plaza)
    l.rect(5,4,9,10,replace(plaza,label='welcome carpet',flat='GVELVET'))
    l.rect(-13,5,-3,13,office)
    l.rect(-3,8,-2,10,replace(office,label='security door',ceil=0,door=True))
    l.rect(-2,8,-1,10,Style('security lintel',ceil=128,roof='GROOF',wall='GTEAL'))
    l.rect(-11,13,-8,18,hall)
    l.rect(-17,18,-3,31,yard)
    # Machinery breaks the yard sightline; a real solid obstacle, not decor collision.
    l.rect(-14,23,-11,26,replace(yard,label='generator',floor=96,flat='GGRATE',wall='GGEN'))
    # Stairs rise east to the booth. Each riser is 16 units, with a 192-unit width.
    for i in range(7):
        l.rect(-3+i,26,-2+i,29,Style(f'staff stair {i+1}', floor=(i+1)*16, flat='GGRATE',wall='GSTEP',light=176))
    l.rect(4,25,11,31,control)
    l.rect(7,24,10,25,replace(control,label='power shutter',ceil=112,door=True,tag=20))
    l.rect(4,21,14,24,balcony)
    # Guardrails are small raised sections, leaving a generous 192-unit drop.
    for x0,x1 in [(4,7),(11,14)]:
        l.rect(x0,21,x1,22,replace(balcony,label='balcony rail',floor=160,flat='GROOF'))
    # Fountain: climbable rim, harmless shallow water and a central plinth.
    l.rect(5,10,11,15,replace(plaza,label='fountain rim',floor=16,flat='GROOF',wall='GTEAL'))
    l.rect(6,11,10,14,replace(plaza,label='fountain water',floor=0,flat='FWATER1',wall='GTEAL',light=192))
    l.rect(7,12,9,13,replace(plaza,label='fountain plinth',floor=80,flat='GROOF',wall='GTEAL'))
    # Ticket kiosks form cover and carnival silhouettes at different heights.
    for x,y in [(0,5),(13,5),(0,16),(14,16)]:
        l.rect(x,y,x+2,y+2,replace(plaza,label='ticket kiosk',floor=112,flat='GREDTOP',wall='GBOOTH'))
    # Blue admission gate, courtyard carousel, flanking closets and final exit.
    l.rect(17,10,18,14,Style('admission lintel',ceil=128,roof='GROOF',wall='GTEAL'))
    l.rect(18,10,19,14,Style('admission gate',ceil=0,roof='GROOF',wall='GBLUE',door=True,tag=10))
    l.rect(19,5,37,21,final)
    l.rect(19,10,20,14,Style('admission porch',ceil=128,roof='GROOF',flat='GCHECK',wall='GTEAL'))
    for i in range(2):
        l.erase(19,5+i,21-i,6+i)
        l.erase(35+i,5+i,37,6+i)
        l.erase(19,20-i,21-i,21-i)
        l.erase(35+i,20-i,37,21-i)
    # Sixteen-sided open ride, radial silk canopy panels and a slim spindle.
    def canopy(height):
        return lambda i: replace(final,label=f'carousel canopy {height} '+('red' if i%2==0 else 'cream'),
                                 floor=32,ceil=height,roof='GREDTOP' if i%2==0 else 'GCREAM',
                                 flat='GPLANK',wall='GVALANCE',light=224 if height==176 else 192)
    middle=replace(final,label='carousel horses',floor=32,ceil=200,roof='GREDTOP',flat='GPLANK',wall='GVALANCE',light=208)
    l.round_area(28,13,(24,9,32,17),[
        (256,final),
        (224,replace(final,label='carousel lit platform',floor=16,flat='GCHECK',wall='SLADRIP1',light=224)),
        (144,canopy(176)),(80,middle),(32,canopy(224)),
        (0,replace(final,label='carousel spindle',floor=224,wall='GPOLE',flat='GROOF',light=192))],final)
    # Wooden carousel horses on brass mounts, with open paths between them.
    for a,b in [((27.5,14.75),(28.5,14.75)),((29.75,12.5),(29.75,13.5)),
                ((28.5,11.25),(27.5,11.25)),((26.25,13.5),(26.25,12.5))]:
        l.panel(a,b,'GHORSE',middle,blocking=True)
        l.edge(a,b,mid='GHORSE',flags=1|16,offset=0,width=64)
    l.rect(23,22,28,26,Style('north maintenance',ceil=128,roof='GROOF',flat='GGRATE',wall='GMETAL',light=128))
    l.rect(24,21,27,22,Style('north ambush door',ceil=0,roof='GROOF',door=True,tag=30))
    l.rect(31,0,36,4,Style('south maintenance',ceil=128,roof='GROOF',flat='GGRATE',wall='GMETAL',light=128))
    l.rect(32,4,35,5,Style('south ambush door',ceil=0,roof='GROOF',door=True,tag=30))
    l.rect(37,12,42,16,Style('midway exit',ceil=128,roof='GROOF',flat='GCARPET',wall='GSTRIPE',light=208))
    # Three useful, optional secrets. Each uses exactly one special-9 sector.
    l.rect(12,-4,18,2,Style('prize garden',ceil=128,flat='GPLANK',wall='GSTRIPE',special=9,light=192))
    l.rect(14,2,16,3,Style('garden door',ceil=0,roof='GROOF',wall='GSTRIPE',door=True))
    l.rect(12,2,14,3,Style('garden window',floor=32,ceil=160,roof='GROOF',wall='GTEAL'))
    for x in range(12,14):
        l.edge((x,3),(x+1,3),mid='GFENCE',flags=1|16,offset=(x-12)*64)
    l.rect(11,-4,12,0,Style('arrival fence',floor=32,ceil=160,roof='GROOF',wall='GTEAL'))
    for y in range(-4,0):
        l.edge((11,y),(11,y+1),mid='GFENCE',flags=1|16)
    l.rect(-18,6,-14,11,Style('snack cache',ceil=128,roof='GROOF',flat='GCHECK',wall='GWOOD',special=9,light=192))
    l.rect(-14,7,-13,9,Style('snack door',ceil=0,roof='GROOF',wall='GSTRIPE',door=True))
    l.rect(-22,22,-18,27,Style('target prize',ceil=128,roof='GROOF',flat='GCHECK',wall='GSTRIPE',special=9,light=192))
    l.rect(-18,23,-17,25,Style('target shutter',ceil=0,roof='GROOF',door=True,tag=40))
    # The power switch also releases a small service-yard guard booth.
    l.erase(-8,18,-3,23)
    l.rect(-7,19,-3,22,Style('staff ambush',ceil=128,roof='GROOF',flat='GGRATE',wall='GMETAL',light=128))
    # This is an enclosed building inset into the yard, with only one entrance.
    for y in range(19,22):
        l.erase(-8,y,-7,y+1)
    l.rect(-8,20,-7,22,Style('staff ambush shutter',ceil=0,roof='GROOF',door=True,tag=20))
    # Lower, framed facades keep the tall marquee and canopy visually dominant.
    for (x,y),style in list(l.cells.items()):
        if style==plaza and any((x+dx,y+dy) not in l.cells for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]):
            l.cells[x,y]=replace(plaza,label='park facade',ceil=176,wall='GWOOD' if (x+y)%5==0 else 'GFACADE',flat='GPLANK',light=144)
        if style==final and any((x+dx,y+dy) not in l.cells for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]):
            l.cells[x,y]=replace(final,label='courtyard facade',ceil=160,wall='GWOOD' if (x+y)%5==0 else 'GFACADE',light=144)
    # Thin masked valances rather than low sky/ceiling slabs above the booths.
    for x,front in [(0,8),(13,8),(0,15),(14,15)]:
        a,b=(x,front),(x+2,front)
        l.panel(a,b,'GFRINGE',plaza)
        l.edge(a,b,mid='GFRINGE',flags=16,offset=0,width=128)
    for y in [14,17]:
        l.rect(-11,y,-8,y+1,replace(hall,label='staff ceiling light',roof='GLAMP',light=192))
    l.rect(-16,21,-15,22,replace(yard,label='stacked crate',floor=48,wall='GCRATE',flat='GPLANK'))
    l.rect(-15,21,-14,22,replace(yard,label='low crate',floor=24,wall='GCRATE',flat='GPLANK'))
    # A slim round finial replaces the fountain's broad gray block.
    water=replace(plaza,label='fountain water',floor=0,flat='FWATER1',wall='GTEAL',light=208)
    l.rect(7,12,9,13,water)
    l.round_area(8,12.5,(7,11,9,14),[(40,water),
        (24,replace(plaza,label='fountain bronze base',floor=16,flat='GROOF',wall='GPOLE',light=192)),
        (0,replace(plaza,label='fountain finial',floor=88,flat='GROOF',wall='GPOLE',light=192))],water,sides=12)
    l.sign((-2,12),(-2,14),'GWINDOW')
    l.sign((18,18),(18,16),'GWINDOW')
    l.sign((28,21),(30,21),'GRIDE')
    l.sign((31,21),(35,21),'GSCREAM')
    for x in range(31,35):
        l.cells[x,20]=replace(l.cells[x,20],label='customer care sign light',light=208)
    # Signs occupy whole 128-unit panels, with continuous texture offsets.
    l.sign((3,-8),(1,-8),'GSAFETY')
    l.sign((7,-8),(5,-8),'GOPEN')
    l.sign((-6,13),(-4,13),'GSECURE')
    l.sign((-13,9),(-13,11),'GSNACK')
    l.sign((-15,31),(-13,31),'GSICK')
    l.sign((11,29),(11,27),'SW1COMM',103,20)
    l.sign((-17,27),(-17,29),'SW1GARG',46,40)
    l.sign((16,-4),(14,-4),'GPRIZE')
    l.sign((42,15),(42,13),'SW1EXIT',11,0)
    for x in range(5,9):
        l.edge((x,2),(x+1,2),mid='GSTFONT1',offset=(x-5)*64,width=256)
    for y in range(10,14):
        l.edge((21,y),(21,y+1),special=2,tag=30,label='courtyard ambush')
    # Four co-op starts; deathmatch starts and spare weapons are flagged separately.
    for i,(x,y) in enumerate([(5.5,-5.5),(7.5,-5.5),(5.5,-6.5),(7.5,-6.5)],1):
        l.thing(x,y,i,angle=90)
    for x,y,a in [(3.5,8.5,0),(15.5,18.5,180),(-15.5,28.5,0),(34.5,18.5,180)]:
        l.thing(x,y,11,angle=a)
    # Guaranteed main-route kit, generous enough to play without secrets.
    pickups = [(7.5,4.5,2007),(-8.5,8.5,2001),(-8.5,10.5,2049),(-6.5,6.5,2018),
               (-10.5,11.5,2011),(-9.5,16.5,2008),(-15.5,19.5,2008),(-15.5,28.5,2049),
               (-5.5,28.5,2011),(5.5,27.5,2011),(8.5,29.5,5),(9.5,26.5,2008),
               (11.5,22.5,2008),(15.5,11.5,2012),(15.5,12.5,2049),(22.5,8.5,2008),
               (22.5,17.5,2011),(34.5,8.5,2049),(35.5,17.5,2012),(39.5,13.5,2008)]
    for x,y,t in pickups:
        l.thing(x,y,t)
    # Extra recovery on Easy; armor remains available before the final fight.
    for x,y,t in [(-6.5,11.5,2012),(-15.5,26.5,2012),(16.5,12.5,2018),(34.5,18.5,2012)]:
        l.thing(x,y,t,1)
    # Secret rewards and readable breadcrumb trails.
    for x,y,t in [(15.5,-1.5,2019),(-16.5,8.5,2012),(-16.5,9.5,2049),(-20.5,24.5,2002),(-20.5,25.5,2048)]:
        l.thing(x,y,t)
    for x,y in [(12.5,5.5),(13.5,4.5),(14.5,3.5),(14.5,1.5),(15.5,.5)]:
        l.thing(x,y,2014)
    # Low-level enemies teach movement before the busier yard and finale.
    # flags: Easy+HMP+UV=7; HMP+UV=6; UV only=4. 8 means ambush/deaf.
    groups = [
        [(3.5,-1.5,3004,7),(8.5,-2.5,3004,6),(2.5,-6.5,3004,4)],
        [(3.5,7.5,3004,7),(11.5,8.5,3004,6),(2.5,13.5,3001,7),(15.5,14.5,3001,7),
         (3.5,18.5,3004,6),(12.5,18.5,3001,6),(16.5,7.5,3004,4),(1.0,17.0,3001,4),(15.0,17.0,3001,4)],
        [(-10.5,7.5,3004,7),(-5.5,11.5,3001,7),(-5.5,7.5,9,6),(-11.5,11.5,3004,4)],
        [(-9.5,15.5,3001,7),(-14.5,21.5,3004,7),(-15.5,25.5,3001,7),(-9.5,28.5,3002,7),
         (-5.5,30.0,3001,6),(-9.5,23.5,9,6),(-15.5,29.5,3004,6),(-4.5,23.5,3004,4),
         (-12.5,27.5,3001,4),(-16.0,22.5,9,4)],
        [(-5.5,20.5,3001,7),(-4.5,19.5,3004,6),(-4.5,21.0,9,4)],
        [(5.5,29.5,3001,7),(9.5,30.0,3004,6),(5.5,25.5,9,4)],
        [(22.5,12.5,3004,7),(26.25,14.75,3001,7),(29.75,11.25,3001,7),
         (34.5,12.5,3004,6),(26.25,11.25,3002,6),(29.75,14.75,3001,6),
         (22.5,6.5,9,4),(35.5,19.0,3001,4),(25.5,8.5,3004,4),(33.5,6.5,3002,4)],
        [(24.5,23.5,3001,7),(26.5,24.5,3004,7),(24.5,25.0,9,6),(27.0,23.0,3001,4)],
        [(32.5,2.5,3001,7),(34.5,1.5,3004,7),(34.5,3.0,9,6),(31.5,1.5,3001,4)],
        [(40.5,14.5,3001,7),(38.5,15.5,3004,6)]
    ]
    for gi, group in enumerate(groups):
        for x,y,t,f in group:
            l.thing(x,y,t,f | (8 if gi in (2,4,5,7,8,9) else 0),angle=270 if y>20 else 180)
    # Barrels sit clear of the route and offer optional combat shortcuts.
    for x,y in [(-15.5,23.5),(-9.5,25.5),(23.5,23.0),(33.0,1.5)]:
        l.thing(x,y,2035)
    # Sparse stock scenery, with no candles/trees obstructing narrow passages.
    # Passage lighting comes from sector/ceiling fixtures, keeping lanes clear.
    l.thing(-15.5,30.0,15)
    l.thing(3.5,-6.5,15)
    l.landmarks = dict(start=[352,-352],shotgun=[-544,544],blue_key=[544,1888],
                       power_switch=[688,1792],balcony_drop=[608,1344],gate=[1152,768],
                       exit=[2672,896],target=[-1088,1792])
    # A shared high sky plane cannot cut across rooms seen from the stairs
    # or balcony. Covered ceilings remain architectural ceilings.
    def high_sky(style):
        return replace(style,ceil=320) if style.roof=='F_SKY1' else style
    l.cells={point:high_sky(style) for point,style in l.cells.items()}
    l.extra_polygons=[(poly,high_sky(style)) for poly,style in l.extra_polygons]
    l.extra_panels=[(a,b,texture,high_sky(style),blocking) for a,b,texture,style,blocking in l.extra_panels]
    # Use the existing boundary wall as a normal upper/lower wall face.
    # A closed backing sector outside the playable map splits that same face
    # at sign height, so the poster appears once without any overlaid surface.
    for key,(texture,offset,special,tag) in list(l.wall_overrides.items()):
        a,b=tuple(key)
        dx,dy=b[0]-a[0],b[1]-a[1]
        length=math.hypot(dx,dy)
        options=[]
        for direction in [-1,1]:
            nx,ny=direction*dy/length,direction*-dx/length
            cell=(math.floor((a[0]+b[0])/2+nx/64),math.floor((a[1]+b[1])/2+ny/64))
            if cell in l.cells:options.append((nx,ny,l.cells[cell]))
        if len(options)!=1 or options[0][2].roof!='F_SKY1':continue
        nx,ny,style=options[0]
        outer=(math.floor((a[0]+b[0])/2-nx/64),math.floor((a[1]+b[1])/2-ny/64))
        height=160 if texture=='GSCREAM' else 128
        l.cells[outer]=Style('sign backing '+texture,floor=height,ceil=height,
                             roof='GROOF',wall=style.wall,light=style.light)
    return l


def make_music():
    # Original instrumental park jingle: every lead note belongs to a lyric
    # syllable. Repeated rhythm, paired phrases and cadences replace the old
    # continuous arpeggio. The source score is deliberately human-editable.
    song = json.loads((HERE/'assets/admission-jingle.json').read_text())
    assert song['meter'] == '3/4'
    beat = round(140*60/song['tempo_bpm'])
    line_beats = song['beats_per_line']
    onsets = song['syllable_onsets_beats']
    assert line_beats == 6 and len(onsets) == 7
    events = defaultdict(list)
    programs = [30,29,34,82]  # Distorted lead, overdriven rhythm, pick bass, ghost calliope.
    instruments = programs + [128+36,128+38,128+42,128+49]
    for ch,p in enumerate(programs):
        events[0].append(bytes([0x40 | ch,0,p]))
    chords = {
        'Am': (45,[57,60,64]),
        'C':  (48,[55,60,64]),
        'Dm': (38,[57,62,65]),
        'E7': (40,[56,59,62,64]),
        'F':  (41,[57,60,65]),
    }
    def pitch(key):
        letters = {'C':0,'D':2,'E':4,'F':5,'G':7,'A':9,'B':11}
        accidental = (1 if '#' in key else -1 if 'b' in key else 0)
        result = 12*(int(key[-1])+1)+letters[key[0]]+accidental
        assert 0 <= result < 128
        return result
    def note(t,ch,p,duration,volume):
        assert 0 <= p < 128 and duration > 0
        events[t].append(bytes([0x10|ch,p|128,volume]))
        events[t+duration].append(bytes([ch,p]))
    cue_lines = [song['title'], 'Original metal carnival jingle for Graveyard Shift',
                 f"{song['tempo_bpm']} BPM | 3/4 | A minor | one note per syllable", '']
    lyric_lines = [song['title'], '']
    for number,line in enumerate(song['lines']):
        assert len(line['syllables']) == len(line['notes']) == len(onsets)
        assert len(line['chords']) == 2
        start = number*line_beats*beat
        lyric_lines.append(line['text'])
        if number == 3:
            lyric_lines.append('')
        cue_lines += [line['text'],
                      'Syllables: ' + ' | '.join(line['syllables']),
                      'Notes:     ' + ' | '.join(line['notes']),
                      'Beats:     ' + ' | '.join(str(x+1) for x in onsets),
                      'Chords:    ' + ' / '.join(line['chords']), '']
        for bar,key in enumerate(line['chords']):
            t = start+bar*3*beat
            bass,_ = chords[key]
            # Tight eighth-note power-chord chugs; short gates suggest palm muting.
            for pulse in range(6):
                for p in [bass,bass+7,bass+12]:
                    note(t+round(pulse*.5*beat),1,p,round(.23*beat),65 if pulse in (0,3) else 53)
            note(t,2,bass,round(1.35*beat),67)
            note(t+round(1.5*beat),2,bass+7,round(1.30*beat),61)
            # 3/4 metal groove: kick, snare backbeats, eighth-note hats.
            for pulse in range(6):
                note(t+round(pulse*.5*beat),15,42,round(.10*beat),44 if pulse%2==0 else 32)
            for onset in [0,1.5,2.5]:
                note(t+round(onset*beat),15,36,round(.18*beat),88)
            for onset in [1,2]:
                note(t+round(onset*beat),15,38,round(.18*beat),76)
            if bar==0 and number in (0,4):
                note(t,15,49,round(1.8*beat),67)
        for i,(syllable,key,onset) in enumerate(zip(line['syllables'],line['notes'],onsets)):
            duration = ((onsets[i+1]-onset)-.08 if i+1<len(onsets)
                        else song['final_note_beats'])
            assert onset+duration <= line_beats
            # Word stress and longer rhyme notes make the imaginary singing
            # audible, even though the in-game score remains instrumental.
            velocity = 85 if i in (0,2,4,6) else 72
            note(start+round(onset*beat),0,pitch(key),round(duration*beat),velocity)
            if i==6:
                note(start+round(onset*beat),3,pitch(key)+12,round(duration*beat),25)
    end = len(song['lines'])*line_beats*beat
    assert all(t <= end for t in events), 'a note spills over the loop boundary'
    events[end].append(bytes([0x60]))
    ticks = sorted(events)
    score = bytearray()
    for i,t in enumerate(ticks):
        es = events[t]
        for j,event in enumerate(es):
            last = j == len(es)-1 and i < len(ticks)-1
            score.extend(bytes([event[0] | (128 if last else 0)])+event[1:])
            if last:
                delta = ticks[i+1]-t
                encoded = [delta & 127]
                delta >>= 7
                while delta:
                    encoded.append((delta & 127)|128)
                    delta >>= 7
                score.extend(reversed(encoded))
    header = struct.pack('<4s6H',b'MUS\x1a',len(score),16+2*len(instruments),4,0,len(instruments),0)
    raw = header+b''.join(struct.pack('<H',p) for p in instruments)+score
    (HERE/'assets/admission-waltz.mus').write_bytes(raw)
    (HERE/'assets/admission-jingle-lyrics.txt').write_text('\n'.join(lyric_lines)+'\n')
    (HERE/'assets/admission-jingle-score.txt').write_text('\n'.join(cue_lines)+'\n')
    return raw


def validate_layout(l):
    # Check placements and broad route connectivity without pretending this is
    # combat playtesting. Runtime movement tests separately verify the route.
    blocking = {1:16,2:16,3:16,4:16,3004:20,9:20,3001:20,3002:30,2035:10,2028:16}
    for x,y,a,t,f in l.things:
        cell = (x//64,y//64)
        assert cell in l.cells, ('thing outside map',t,x,y)
        s = l.cells[cell]
        assert s.ceil-s.floor >= 56, ('thing in low sector',t,x,y,s.label)
        if t in blocking:
            radius = blocking[t]
            for dx,dy in [(-radius,-radius),(-radius,radius),(radius,-radius),(radius,radius)]:
                c = ((x+dx)//64,(y+dy)//64)
                assert c in l.cells, ('thing against wall',t,x,y,c)
                neighbor = l.cells[c]
                assert neighbor.floor <= s.floor+24 and neighbor.ceil >= s.floor+56, ('thing overlaps obstacle',t,x,y,neighbor.label)
    # Reachability with all doors open, respecting 24-unit stairs and headroom.
    start = (5,-6)
    seen = {start}
    todo = deque([start])
    while todo:
        p = todo.popleft()
        s = l.cells[p]
        for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]:
            q = (p[0]+dx,p[1]+dy)
            if q not in l.cells or q in seen:
                continue
            n = l.cells[q]
            if n.floor-s.floor > 24 or (not n.door and n.ceil-max(s.floor,n.floor)<56):
                continue
            # Window/fence cells cannot provide alternate access to the garden.
            if n.label in ('garden window','arrival fence') or s.label in ('garden window','arrival fence'):
                continue
            seen.add(q)
            todo.append(q)
    for label, point in l.landmarks.items():
        if label in ('power_switch','gate','exit','target'):
            continue
        assert (point[0]//64,point[1]//64) in seen,(label,'unreachable')
    assert (40,14) in seen, 'exit unreachable'


def overview(l):
    scale = 12
    xs, ys = zip(*l.cells)
    minx,maxx,miny,maxy = min(xs),max(xs)+1,min(ys),max(ys)+1
    w,h = (maxx-minx)*scale+100,(maxy-miny)*scale+180
    im = Image.new('RGB',(w,h),INK)
    d = ImageDraw.Draw(im)
    def pos(x,y):
        return (50+(x-minx)*scale,100+(maxy-y)*scale)
    colors = {'arrival':(56,62,74),'security office':(61,118,111),'service yard':(87,96,95),
              'control booth':(80,146,136),'ticket plaza':(166,111,88),'welcome courtyard':(152,87,98)}
    for (x,y),s in l.cells.items():
        color = colors.get(s.label, (115,106,101))
        if s.special == 9:
            color = (152,129,62)
        elif s.door:
            color = (57,94,168) if s.label == 'admission gate' else (202,159,79)
        elif s.floor >= 64:
            color = (87,104,119)
        a,b = pos(x,y+1),pos(x+1,y)
        d.rectangle((*a,*b),fill=color)
        for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]:
            if (x+dx,y+dy) in l.cells:
                continue
            if dx:
                px = x+(dx>0)
                d.line((*pos(px,y),*pos(px,y+1)),fill=CREAM,width=1)
            else:
                py = y+(dy>0)
                d.line((*pos(x,py),*pos(x+1,py)),fill=CREAM,width=1)
    for x,y,a,t,f in l.things:
        px,py = pos(x/64,y/64)
        if t in (3004,9,3001,3002):
            d.ellipse((px-2,py-2,px+2,py+2),fill=(224,86,74) if f&2 else (117,54,61))
        elif t in (2001,2002,5,2019,1):
            d.rectangle((px-3,py-3,px+3,py+3),fill=GOLD if t != 1 else (115,232,165))
    labels = [(5.5,-7,'ARRIVAL'),(7.5,18.5,'TICKET PLAZA'),(-8,6,'SECURITY'),(-10,30,'STAFF YARD'),
              (7.5,27,'CONTROL'),(9,22.5,'BALCONY'),(28,19.5,'WELCOME COURTYARD'),(39,14,'EXIT')]
    for x,y,label in labels:
        px,py = pos(x,y)
        f = font(10)
        box = d.textbbox((0,0),label,font=f)
        tw = box[2]
        d.rectangle((px-tw/2-3,py-7,px+tw/2+3,py+7),fill=INK)
        d.text((px-tw/2,py-6),label,font=f,fill=CREAM)
    d.text((35,24),'GRAVEYARD SHIFT',font=font(25),fill=CREAM)
    d.text((35,59),'01 / ADMISSION IS MANDATORY',font=font(14),fill=GOLD)
    d.text((35,h-49),'Green: start   Gold: pickup / secret / shutter   Red: HMP enemy',font=font(11),fill=CREAM)
    d.text((35,h-29),'Classic DOOM format / original map, textures and music',font=font(11),fill=(161,160,163))
    im.save(HERE/'previews/map-overview.png')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad',type=Path,default=ROOT/'DOOM1.WAD')
    parser.add_argument('--node-builder',default=os.environ.get('ZDBSP','zdbsp'),help='ZDBSP executable for classic node building')
    parser.add_argument('--keep-art',action='store_true',help='Reuse edited PNGs instead of regenerating them')
    args = parser.parse_args()
    (HERE/'previews').mkdir(parents=True,exist_ok=True)
    base = read_wad(args.iwad)
    art = make_art(args.keep_art)
    level = make_level()
    validate_layout(level)
    lumps = level.build()+art_lumps(art,base)+[('D_E1M1',make_music())]
    # Remove invisible construction seams before building the final BSP.
    raw=dict(lumps)
    sides=list(struct.iter_unpack('<hh8s8s8sH',raw['SIDEDEFS']))
    kept=[];line_remap={}
    for old_index,line in enumerate(struct.iter_unpack('<7H',raw['LINEDEFS'])):
        a,b,flags,special,tag,front,back=line
        if back!=65535 and sides[front][5]==sides[back][5] and not special and not tag and all(sides[i][4].rstrip(b'\0')==b'-' for i in [front,back]):continue
        line_remap[old_index]=len(kept)
        kept.append(struct.pack('<7H',*line))
    level.manifest['actions']={key:[line_remap[i] for i in value] for key,value in level.manifest['actions'].items()}
    lumps=[(key,b''.join(kept) if key=='LINEDEFS' else data) for key,data in lumps]
    builder=shutil.which(args.node_builder)
    if not builder:
        raise SystemExit('ZDBSP is required. Install zdbsp or pass --node-builder /path/to/zdbsp')
    with tempfile.TemporaryDirectory(prefix='graveyard-nodes-') as temp:
        source=Path(temp)/'source.wad';output=Path(temp)/'finished.wad'
        write_wad(lumps,source)
        subprocess.run([builder,'-q','-R','-t','-o',str(output),str(source)],check=True)
        cooked=read_wad(output)
        assert not cooked['NODES'].startswith((b'XNOD',b'ZNOD')), 'unexpected extended nodes'
        (HERE/'graveyard-shift.wad').write_bytes(output.read_bytes())
        level.manifest.update(nodes=len(cooked['NODES'])//28,subsectors=len(cooked['SSECTORS'])//4,
                              linedefs=len(cooked['LINEDEFS'])//14,vertices=len(cooked['VERTEXES'])//4)

    (HERE/'map-manifest.json').write_text(json.dumps(level.manifest,indent=2)+'\n')
    overview(level)
    print(json.dumps(level.manifest,indent=2))
    print(f'Built {HERE / "graveyard-shift.wad"}')


if __name__ == '__main__':
    main()
