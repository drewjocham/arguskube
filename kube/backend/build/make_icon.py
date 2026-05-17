#!/usr/bin/env python3
"""
Generate the Argus app icon — a honey badger face emblem.

Why a honey badger? Argus watches Kubernetes clusters with the same
unflappable obstinance a honey badger gives a beehive: persistent,
fearless, and uninterested in being told the task is too hard.

The signature visual differentiator from a European badger is the
single broad white "cape" sweeping over the top of the head (not the
two parallel white stripes of the European species). The icon leans
on that one feature plus a hint of bared teeth so the silhouette
still reads at the 16×16 Finder thumbnail.

Rendered at 1024×1024 RGBA so macOS's iconutil / Wails can downscale
to the full asset family without re-rasterizing.

Run:  python3 make_icon.py
Out:  appicon.png  (overwrites the existing one)
"""
from PIL import Image, ImageDraw, ImageFilter
from pathlib import Path

SIZE = 1024
HERE = Path(__file__).resolve().parent
OUT = HERE / "appicon.png"

# Palette — picked to sit comfortably on both a light Dock and a dark
# menu bar. The background slate matches the LoginView gradient so
# the icon reads as "Argus" before the user sees the wordmark.
BG_TOP    = (32, 36, 42, 255)
BG_BOTTOM = (14, 16, 18, 255)
ACCENT    = (240, 196, 84, 255)   # warm amber — the badger's "eye spark"
FUR_BLACK = (24, 24, 26, 255)
FUR_WHITE = (240, 240, 236, 255)
TOOTH     = (245, 244, 240, 255)
NOSE      = (24, 24, 26, 255)


def rounded_rect_gradient():
    """The background tile — vertical gradient, big rounded corners."""
    img = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    grad = Image.new("RGBA", (1, SIZE))
    for y in range(SIZE):
        t = y / (SIZE - 1)
        r = int(BG_TOP[0] * (1 - t) + BG_BOTTOM[0] * t)
        g = int(BG_TOP[1] * (1 - t) + BG_BOTTOM[1] * t)
        b = int(BG_TOP[2] * (1 - t) + BG_BOTTOM[2] * t)
        grad.putpixel((0, y), (r, g, b, 255))
    grad = grad.resize((SIZE, SIZE))

    mask = Image.new("L", (SIZE, SIZE), 0)
    md = ImageDraw.Draw(mask)
    # Apple's app icon mask is approximately a "squircle" — a rounded
    # rectangle with corner radius ≈ 22.5% of the side. PIL's
    # rounded_rectangle is close enough for our purposes.
    radius = int(SIZE * 0.225)
    md.rounded_rectangle((0, 0, SIZE, SIZE), radius=radius, fill=255)

    img.paste(grad, (0, 0), mask)
    return img


def honey_badger_face(img):
    """Draw the honey badger emblem centred on the background tile."""
    d = ImageDraw.Draw(img)
    cx, cy = SIZE // 2, SIZE // 2

    # --- Black head silhouette ---------------------------------------
    # The honey badger reads as a wide, low head — wider than a
    # European badger's pointed muzzle, with a soft squarish jaw.
    # Composed as a rounded rectangle (cranium) + a slightly wider
    # rounded rectangle below (jowls) so the bottom doesn't taper.
    head_w = int(SIZE * 0.66)
    head_h = int(SIZE * 0.62)
    head_box = (cx - head_w // 2, cy - head_h // 2 + 20,
                cx + head_w // 2, cy + head_h // 2 + 20)
    d.rounded_rectangle(head_box, radius=int(SIZE * 0.20), fill=FUR_BLACK)

    jowl_w = int(SIZE * 0.56)
    jowl_h = int(SIZE * 0.22)
    jowl_box = (cx - jowl_w // 2,
                cy + head_h // 2 - jowl_h // 2 - 10,
                cx + jowl_w // 2,
                cy + head_h // 2 + jowl_h // 2 - 10)
    d.rounded_rectangle(jowl_box, radius=int(SIZE * 0.16), fill=FUR_BLACK)

    # --- Signature white cape ----------------------------------------
    # The defining visual: a single broad arch of white from temple to
    # temple, sweeping over the top of the head. Drawn as a rounded
    # rectangle covering the upper-third of the head with the bottom
    # corners softened so it reads as a hood, not a hat. A small
    # amount of black above the eye-line keeps the brow visible.
    cape_w = head_w - int(SIZE * 0.06)
    cape_top = head_box[1] - int(SIZE * 0.06)
    cape_bottom = cy - int(SIZE * 0.04)
    d.rounded_rectangle(
        (cx - cape_w // 2, cape_top,
         cx + cape_w // 2, cape_bottom),
        radius=int(SIZE * 0.22),
        fill=FUR_WHITE,
    )
    # Tuck the cape's bottom edge down into a soft "V" over the
    # forehead — without this the bottom edge looks like a hat brim.
    # Two small black triangles on either side of the centre line
    # bring the underside of the cape to a gentle peak.
    peak_w = int(SIZE * 0.28)
    peak_h = int(SIZE * 0.06)
    d.polygon([
        (cx - peak_w // 2, cape_bottom - 2),
        (cx + peak_w // 2, cape_bottom - 2),
        (cx, cape_bottom + peak_h),
    ], fill=FUR_BLACK)

    # --- Small dark ears poking through the cape ---------------------
    # Just enough silhouette to register at small sizes without
    # breaking the clean cape arch.
    ear_inset_x = int(SIZE * 0.10)
    ear_top_y   = head_box[1] - int(SIZE * 0.05)
    ear_base_y  = head_box[1] + int(SIZE * 0.04)
    ear_half_w  = int(SIZE * 0.045)
    for side in (-1, +1):
        ear_cx = cx + side * (head_w // 2 - ear_inset_x)
        d.polygon([
            (ear_cx - ear_half_w, ear_base_y),
            (ear_cx + ear_half_w, ear_base_y),
            (ear_cx, ear_top_y),
        ], fill=FUR_BLACK)

    # --- Amber eyes inside the black face ----------------------------
    # Set wide; the honey badger's stare reads more aggressive than
    # the European badger's quizzical squint.
    eye_r = int(SIZE * 0.030)
    eye_y = cy + int(SIZE * 0.03)
    eye_dx = int(SIZE * 0.11)
    for side in (-1, +1):
        cx_e = cx + side * eye_dx
        d.ellipse(
            (cx_e - eye_r, eye_y - eye_r,
             cx_e + eye_r, eye_y + eye_r),
            fill=ACCENT,
        )

    # --- Nose --------------------------------------------------------
    nose_w = int(SIZE * 0.11)
    nose_h = int(SIZE * 0.07)
    nose_cy = cy + int(SIZE * 0.16)
    d.rounded_rectangle(
        (cx - nose_w // 2, nose_cy - nose_h // 2,
         cx + nose_w // 2, nose_cy + nose_h // 2),
        radius=int(nose_h * 0.45),
        fill=NOSE,
    )

    # --- Bared-teeth detail ------------------------------------------
    # Two tiny white fang shapes hanging just below the muzzle line.
    # This is the "honey badger don't care" tell — a European badger
    # icon would not include this. Sized to disappear gracefully at
    # 16×16 (the fangs collapse to a single white sliver, which still
    # reads as "open mouth").
    fang_w = int(SIZE * 0.024)
    fang_h = int(SIZE * 0.055)
    fang_y = nose_cy + int(SIZE * 0.05)
    fang_dx = int(SIZE * 0.035)
    for side in (-1, +1):
        fang_cx = cx + side * fang_dx
        d.polygon([
            (fang_cx - fang_w // 2, fang_y),
            (fang_cx + fang_w // 2, fang_y),
            (fang_cx, fang_y + fang_h),
        ], fill=TOOTH)

    return img


def main():
    bg = rounded_rect_gradient()
    icon = honey_badger_face(bg)
    # A whisper of inner shadow against the squircle edge — pure
    # decoration, kept subtle so the icon doesn't look "graphic-designed".
    glow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    radius = int(SIZE * 0.225)
    gd.rounded_rectangle((6, 6, SIZE - 6, SIZE - 6),
                         radius=radius, outline=(0, 0, 0, 60), width=12)
    glow = glow.filter(ImageFilter.GaussianBlur(6))
    icon = Image.alpha_composite(icon, glow)

    icon.save(OUT, "PNG")
    print(f"wrote {OUT} ({icon.size[0]}×{icon.size[1]})")


if __name__ == "__main__":
    main()
