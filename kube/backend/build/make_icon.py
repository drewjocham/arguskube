#!/usr/bin/env python3
"""
Generate the Argus app icon — a clean, minimal badger face emblem.

Why a badger? Argus watches over Kubernetes clusters with the same
patient, no-nonsense attention a badger gives its sett: persistent,
unflappable, and unimpressed by surface drama. The two white face
stripes are the signature visual feature, so the icon leans on them
rather than fine detail that won't survive the 16×16 Finder thumbnail.

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
# menu bar. The background is the same slate the LoginView gradient
# lands on, so the icon reads as "Argus" before the user even sees
# the wordmark.
BG_TOP    = (32, 36, 42, 255)
BG_BOTTOM = (14, 16, 18, 255)
ACCENT    = (240, 196, 84, 255)   # warm amber — the badger's "eye spark"
FUR_BLACK = (24, 24, 26, 255)
FUR_WHITE = (240, 240, 236, 255)
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


def badger_face(img):
    """Draws the badger emblem centred on the background tile."""
    d = ImageDraw.Draw(img)
    cx, cy = SIZE // 2, SIZE // 2

    # --- Head silhouette (white) -------------------------------------
    # A slightly elongated wedge: wider at the brow, tapering to a
    # rounded muzzle. We compose it from a rounded rectangle (cranium)
    # plus a downward-pointing chord (muzzle) to avoid the "pizza
    # slice" look a pure triangle gives at icon sizes.
    head_w = int(SIZE * 0.62)
    head_h = int(SIZE * 0.74)
    head_box = (cx - head_w // 2, cy - head_h // 2 + 30,
                cx + head_w // 2, cy + head_h // 2 + 30)
    d.rounded_rectangle(head_box, radius=int(SIZE * 0.22), fill=FUR_WHITE)

    # Muzzle: a smaller rounded rect overlapping the lower face, so the
    # bottom reads as a softer chin than the cranium rectangle alone.
    muzzle_w = int(SIZE * 0.36)
    muzzle_h = int(SIZE * 0.30)
    muzzle_box = (cx - muzzle_w // 2,
                  cy + head_h // 2 - muzzle_h // 2 - 40,
                  cx + muzzle_w // 2,
                  cy + head_h // 2 + muzzle_h // 2 - 40)
    d.rounded_rectangle(muzzle_box, radius=int(SIZE * 0.18), fill=FUR_WHITE)

    # --- Ears (black, sit on top of the head) ------------------------
    # Two stubby triangles set just inside the cranium edges. We park
    # them well outside the stripe channel so a 32×32 render still
    # reads as "ears + stripes" rather than a single dark slab.
    ear_inset_x = int(SIZE * 0.085)
    ear_top_y   = head_box[1] - int(SIZE * 0.04)
    ear_base_y  = head_box[1] + int(SIZE * 0.08)
    ear_half_w  = int(SIZE * 0.05)
    for side in (-1, +1):
        ear_cx = cx + side * (head_w // 2 - ear_inset_x)
        d.polygon([
            (ear_cx - ear_half_w, ear_base_y),
            (ear_cx + ear_half_w, ear_base_y),
            (ear_cx, ear_top_y),
        ], fill=FUR_BLACK)

    # --- Signature black stripes -------------------------------------
    # Two vertical bars running from the brow down through the eye
    # line, with a narrow white blaze between them.  Width and spacing
    # are deliberately chunky so the icon survives at 32×32.
    stripe_w = int(SIZE * 0.085)
    stripe_top    = head_box[1] - int(SIZE * 0.03)
    stripe_bottom = cy + int(SIZE * 0.16)
    blaze_half    = int(SIZE * 0.045)
    for side in (-1, +1):
        cx_s = cx + side * (blaze_half + stripe_w // 2 + int(SIZE * 0.015))
        d.rounded_rectangle(
            (cx_s - stripe_w // 2, stripe_top,
             cx_s + stripe_w // 2, stripe_bottom),
            radius=int(stripe_w * 0.45),
            fill=FUR_BLACK,
        )

    # --- Eyes (amber dots set inside the black stripes) --------------
    eye_r = int(SIZE * 0.022)
    eye_y = cy - int(SIZE * 0.03)
    for side in (-1, +1):
        cx_e = cx + side * (blaze_half + stripe_w // 2 + int(SIZE * 0.015))
        d.ellipse(
            (cx_e - eye_r, eye_y - eye_r,
             cx_e + eye_r, eye_y + eye_r),
            fill=ACCENT,
        )

    # --- Nose ---------------------------------------------------------
    nose_w = int(SIZE * 0.10)
    nose_h = int(SIZE * 0.07)
    nose_cy = cy + int(SIZE * 0.18)
    d.rounded_rectangle(
        (cx - nose_w // 2, nose_cy - nose_h // 2,
         cx + nose_w // 2, nose_cy + nose_h // 2),
        radius=int(nose_h * 0.45),
        fill=NOSE,
    )

    return img


def main():
    bg = rounded_rect_gradient()
    icon = badger_face(bg)
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
