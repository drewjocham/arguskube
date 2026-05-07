import { h } from 'vue'

/**
 * VolumeCylinder — SVG cylinder with animated water wave.
 * Shows capacity usage as a 3D-ish cylinder with a sloshing wave.
 *
 * Props:
 *   pct     — fill percentage (0–1)
 *   color   — CSS color for the water
 *   size    — width in px (height scales proportionally)
 *   showPct — overlay percentage text
 */
const VolumeCylinder = {
  name: 'VolumeCylinder',
  props: {
    pct:    { type: Number, default: 0 },
    color:  { type: String, default: '#3ecf8e' },
    size:   { type: Number, default: 32 },
    showPct: { type: Boolean, default: false },
  },
  render() {
    const { pct, color, size, showPct } = this
    const w = size
    const h = size * 1.3
    const rx = w * 0.25
    const topY = rx * 0.45
    const bodyH = h - rx
    const waterH = Math.max(2, (bodyH - rx) * pct)
    const waterY = h - rx - waterH
    const pctDisplay = Math.round(pct * 100)
    const uid = this._uid || 0

    return h('div', {
      class: 'cylinder-wrap',
      style: `width:${w}px;height:${h}px;position:relative;flex-shrink:0;`
    }, [
      h('svg', { width: w, height: h, viewBox: `0 0 ${w} ${h}`, style: 'display:block;' }, [
        // Background cylinder shell
        h('path', {
          d: `M0,${rx} L0,${h - rx} A${rx},${rx} 0 0,0 ${w},${h - rx} L${w},${rx} A${rx},${rx} 0 0,0 0,${rx} Z`,
          fill: 'rgba(255,255,255,0.04)',
          stroke: 'rgba(255,255,255,0.1)',
          'stroke-width': 1,
        }),
        // Top ellipse
        h('ellipse', {
          cx: w / 2, cy: rx, rx: rx, ry: rx * 0.5,
          fill: 'rgba(255,255,255,0.06)',
          stroke: 'rgba(255,255,255,0.12)',
          'stroke-width': 1,
        }),
        // Clip path for water
        h('defs', [
          h('clipPath', { id: `cyl-${uid}` }, [
            h('path', {
              d: `M0,${rx} L0,${h - rx} A${rx},${rx} 0 0,0 ${w},${h - rx} L${w},${rx} A${rx},${rx} 0 0,0 0,${rx} Z`,
            }),
          ]),
        ]),
        h('g', { 'clip-path': `url(#cyl-${uid})` }, [
          // Water body
          h('rect', {
            x: 0, y: waterY, width: w, height: bodyH,
            fill: color, opacity: 0.7,
          }),
          // Wave layer 1
          h('path', {
            class: 'cylinder-water',
            d: `M0,${waterY + 2} Q${w * 0.25},${waterY - 3} ${w * 0.5},${waterY + 1} T${w},${waterY + 2} L${w},${waterY + 8} Q${w * 0.75},${waterY + 4} ${w * 0.5},${waterY + 6} T0,${waterY + 8} Z`,
            fill: color, opacity: 0.9,
          }),
          // Wave layer 2 (offset, lighter)
          h('path', {
            class: 'cylinder-water-fast',
            d: `M0,${waterY + 5} Q${w * 0.3},${waterY + 1} ${w * 0.5},${waterY + 4} T${w},${waterY + 5} L${w},${waterY + 10} Q${w * 0.7},${waterY + 7} ${w * 0.5},${waterY + 9} T0,${waterY + 10} Z`,
            fill: 'rgba(255,255,255,0.25)',
            opacity: 0.5,
          }),
        ]),
        // Glass highlight
        h('path', {
          d: `M2,${rx + 6} L2,${h - rx - 6}`,
          stroke: 'rgba(255,255,255,0.15)',
          'stroke-width': 1.5,
          'stroke-linecap': 'round',
          fill: 'none',
        }),
      ]),
      showPct
        ? h('div', {
            class: 'pct-label',
            style: `position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font-size:${size * 0.32}px;font-weight:600;font-family:var(--mono);color:#fff;text-shadow:0 1px 3px rgba(0,0,0,0.6);pointer-events:none;`
          }, [`${pctDisplay}%`])
        : null,
    ])
  }
}

export default VolumeCylinder
