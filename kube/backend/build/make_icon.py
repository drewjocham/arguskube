#!/usr/bin/env python3
"""
Generate the Argus app icon — a honey badger face emblem.

Why a honey badger? Argus watches Kubernetes clusters with the same
unflappable obstinance a honey badger gives a beehive: persistent,
fearless, uninterested in being told the task is too hard.

The distinguishing feature from a European badger is the single
broad white cape that runs from the brow over the crown and down
the back. From the head-on portrait angle this icon uses, the cape
shows as a clean half-moon crowning the head — drawn as a PIL chord
(filled semi-ellipse) so its flat bottom lands precisely at the
brow line. No clip-rect tricks; the geometry is the geometry.

Honey badger ears are tiny and often invisible head-on; the icon
omits them rather than fake spikes that would read as horns.

Rendered at 1024×1024 RGBA so macOS's iconutil / Wails can downscale
the full asset family without re-rasterizing. Hand-tested at the
sizes the Dock and Finder use: 16, 32, 64, 128.

Run:  python3 make_icon.py
Out:  appicon.png  (overwrites the existing one)
"""
from PIL import Image, ImageDraw, ImageFilter
from pathlib import Path

SIZE = 1024
HERE = Path(__file__).resolve().parent
OUT = HERE / "appicon.png"

# Palette — slate background matches the LoginView gradient so the
# icon reads as "Argus" before the user sees the wordmark.
BG_TOP    = (32, 36, 42, 255)
BG_BOTTOM = (14, 16, 18, 255)
ACCENT    = (240, 196, 84, 255)   # warm amber eye spark
FUR_BLACK = (24, 24, 26, 255)
FUR_WHITE = (240, 240, 236, 255)
NOSE_PINK = (148, 78, 82, 255)    # muted ox-blood, visible against black


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
    # Apple's app icon mask is approximately a squircle: rounded
    # rectangle with corner radius ≈ 22.5% of the side.
    radius = int(SIZE * 0.225)
    md.rounded_rectangle((0, 0, SIZE, SIZE), radius=radius, fill=255)

    img.paste(grad, (0, 0), mask)
    return img


def honey_badger_face(img):
    """Draw the honey badger head emblem centred on the background tile."""
    d = ImageDraw.Draw(img)
    cx, cy = SIZE // 2, SIZE // 2

    # --- Head silhouette (black) -------------------------------------
    # Cranium: wide rounded rectangle, slightly taller than the
    # muzzle so the silhouette doesn't read as a circle.
    cranium_w = int(SIZE * 0.66)
    cranium_top = cy - int(SIZE * 0.24)
    cranium_bot = cy + int(SIZE * 0.26)
    cranium = (cx - cranium_w // 2, cranium_top,
               cx + cranium_w // 2, cranium_bot)
    d.rounded_rectangle(cranium, radius=int(SIZE * 0.20), fill=FUR_BLACK)

    # Muzzle: a narrower rounded rectangle whose top overlaps the
    # cranium and whose bottom protrudes below it. Gives a "snout"
    # without breaking the head silhouette.
    muzzle_w = int(SIZE * 0.40)
    muzzle = (cx - muzzle_w // 2, cy + int(SIZE * 0.10),
              cx + muzzle_w // 2, cy + int(SIZE * 0.36))
    d.rounded_rectangle(muzzle, radius=int(SIZE * 0.11), fill=FUR_BLACK)

    # --- Signature white cape (chord = half-moon) --------------------
    # The cape's BOUNDING BOX is twice as tall as the visible cape so
    # the chord at the midline lands exactly at the brow. PIL's chord
    # angles run clockwise from the 3-o'clock direction; (180°, 360°)
    # selects the upper half of the ellipse.
    cape_w     = cranium_w + int(SIZE * 0.02)  # cape edge tucks just
                                               # outside the cranium so
                                               # there's no gap at the
                                               # temple corners.
    cape_brow  = cy - int(SIZE * 0.08)         # bottom of the cape
    cape_crown = cranium_top - int(SIZE * 0.02) # top of the cape
    cape_h     = cape_brow - cape_crown
    cape_bbox  = (cx - cape_w // 2, cape_crown,
                  cx + cape_w // 2, cape_crown + cape_h * 2)
    d.chord(cape_bbox, 180, 360, fill=FUR_WHITE)

    # --- Eyes (solid amber, larger + slight forward slant) -----------
    # Earlier drafts had small dots that read as a baby animal. Real
    # honey badgers have a focused, narrow-eyed stare. Drawing the
    # eyes as flattened ellipses (wider than tall) gives that "alert"
    # silhouette without needing finer detail than 32×32 can carry.
    eye_rx = int(SIZE * 0.055)   # wider…
    eye_ry = int(SIZE * 0.038)   # …than tall
    eye_y = cape_brow + int(SIZE * 0.085)
    eye_dx = int(SIZE * 0.115)
    for side in (-1, +1):
        ex = cx + side * eye_dx
        d.ellipse(
            (ex - eye_rx, eye_y - eye_ry,
             ex + eye_rx, eye_y + eye_ry),
            fill=ACCENT,
        )

    # --- Nose (single rounded shape on the muzzle) -------------------
    # Sits at ~65% down the face so it's unambiguously in the snout
    # zone, not the chin. A wider-than-tall ellipse reads as a
    # mustelid rhinarium without the boxy polygon edges the previous
    # composite produced.
    nose_w = int(SIZE * 0.090)
    nose_h = int(SIZE * 0.065)
    nose_cy = cy + int(SIZE * 0.16)
    d.ellipse(
        (cx - nose_w // 2, nose_cy - nose_h // 2,
         cx + nose_w // 2, nose_cy + nose_h // 2),
        fill=NOSE_PINK,
    )

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
