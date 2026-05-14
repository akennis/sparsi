#!/usr/bin/env python3
"""Replace inline nav-logo and footer-logo SVGs in all HTML files with the new DAG logo."""
import glob, os, re

ROOT = os.path.join(os.path.dirname(__file__), "..")

# Layered DAG: L1(x=14, 2 det) -> L2(x=38, 3 det) -> L3(x=62, 2 AI) -> L4(x=86, 1 AI).
# Edges are pre-shortened so they sit between node circumferences (radius 8).
NAV_SVG = (
    '<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="sparsi DAG - 3 AI nodes distributed across left and middle layers">'
    '<defs>'
    '<radialGradient id="dl-d" cx="38%" cy="35%" r="65%"><stop offset="0%" stop-color="#60a5fa"/><stop offset="100%" stop-color="#1e3a8a"/></radialGradient>'
    '<radialGradient id="dl-ai" cx="38%" cy="35%" r="65%"><stop offset="0%" stop-color="#c084fc"/><stop offset="100%" stop-color="#5b21b6"/></radialGradient>'
    '<filter id="dl-glow" x="-60%" y="-60%" width="220%" height="220%"><feGaussianBlur stdDeviation="1.4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'
    '</defs>'
    '<g stroke="#64748b" stroke-width="1.3" fill="none" stroke-linecap="round">'
    '<line x1="21" y1="30" x2="31" y2="26"/>'
    '<line x1="21" y1="38" x2="31" y2="46"/>'
    '<line x1="21" y1="62" x2="31" y2="54"/>'
    '<line x1="21" y1="70" x2="31" y2="74"/>'
    '<line x1="45" y1="26" x2="55" y2="30"/>'
    '<line x1="45" y1="46" x2="55" y2="38"/>'
    '<line x1="45" y1="54" x2="55" y2="62"/>'
    '<line x1="45" y1="74" x2="55" y2="70"/>'
    '<line x1="69" y1="38" x2="79" y2="46"/>'
    '<line x1="69" y1="62" x2="79" y2="54"/>'
    '</g>'
    '<circle cx="38" cy="22" r="8" fill="url(#dl-d)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="38" cy="78" r="8" fill="url(#dl-d)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="62" cy="34" r="8" fill="url(#dl-d)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="62" cy="66" r="8" fill="url(#dl-d)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="86" cy="50" r="8" fill="url(#dl-d)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="14" cy="34" r="8" fill="url(#dl-ai)" stroke="#c084fc" stroke-width="1.2" filter="url(#dl-glow)"/>'
    '<circle cx="14" cy="66" r="8" fill="url(#dl-ai)" stroke="#c084fc" stroke-width="1.2" filter="url(#dl-glow)"/>'
    '<circle cx="38" cy="50" r="8" fill="url(#dl-ai)" stroke="#c084fc" stroke-width="1.2" filter="url(#dl-glow)"/>'
    '</svg>'
)

# Footer DAG: identical topology with separate gradient IDs so it can coexist with the
# nav-logo on the same page; no glow filter (visual is too small to benefit).
FOOTER_SVG = (
    '<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg" width="28" height="28" role="img" aria-label="sparsi DAG">'
    '<defs>'
    '<radialGradient id="dl-fd" cx="38%" cy="35%" r="65%"><stop offset="0%" stop-color="#60a5fa"/><stop offset="100%" stop-color="#1e3a8a"/></radialGradient>'
    '<radialGradient id="dl-fai" cx="38%" cy="35%" r="65%"><stop offset="0%" stop-color="#c084fc"/><stop offset="100%" stop-color="#5b21b6"/></radialGradient>'
    '</defs>'
    '<g stroke="#64748b" stroke-width="1.3" fill="none" stroke-linecap="round">'
    '<line x1="21" y1="30" x2="31" y2="26"/>'
    '<line x1="21" y1="38" x2="31" y2="46"/>'
    '<line x1="21" y1="62" x2="31" y2="54"/>'
    '<line x1="21" y1="70" x2="31" y2="74"/>'
    '<line x1="45" y1="26" x2="55" y2="30"/>'
    '<line x1="45" y1="46" x2="55" y2="38"/>'
    '<line x1="45" y1="54" x2="55" y2="62"/>'
    '<line x1="45" y1="74" x2="55" y2="70"/>'
    '<line x1="69" y1="38" x2="79" y2="46"/>'
    '<line x1="69" y1="62" x2="79" y2="54"/>'
    '</g>'
    '<circle cx="38" cy="22" r="8" fill="url(#dl-fd)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="38" cy="78" r="8" fill="url(#dl-fd)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="62" cy="34" r="8" fill="url(#dl-fd)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="62" cy="66" r="8" fill="url(#dl-fd)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="86" cy="50" r="8" fill="url(#dl-fd)" stroke="#60a5fa" stroke-width="1.2"/>'
    '<circle cx="14" cy="34" r="8" fill="url(#dl-fai)" stroke="#c084fc" stroke-width="1.2"/>'
    '<circle cx="14" cy="66" r="8" fill="url(#dl-fai)" stroke="#c084fc" stroke-width="1.2"/>'
    '<circle cx="38" cy="50" r="8" fill="url(#dl-fai)" stroke="#c084fc" stroke-width="1.2"/>'
    '</svg>'
)

NAV_MARKER = re.compile(r'(class="nav-logo">\s*\n\s*)<svg\b[^<]*(?:<(?!/svg>)[^<]*)*</svg>')
FOOTER_MARKER = re.compile(r'(class="footer-logo">\s*\n\s*)<svg\b[^<]*(?:<(?!/svg>)[^<]*)*</svg>')

changed = 0
for path in sorted(glob.glob(os.path.join(ROOT, "*.html"))):
    with open(path, "r", encoding="utf-8") as f:
        src = f.read()
    new = NAV_MARKER.sub(r'\1' + NAV_SVG, src)
    new = FOOTER_MARKER.sub(r'\1' + FOOTER_SVG, new)
    if new != src:
        with open(path, "w", encoding="utf-8") as f:
            f.write(new)
        changed += 1
        print(f"  updated {os.path.basename(path)}")
    else:
        print(f"  unchanged {os.path.basename(path)}")
print(f"\n{changed} file(s) updated")
