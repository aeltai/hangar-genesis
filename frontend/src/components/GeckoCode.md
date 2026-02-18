# Gecko (turtle) character – full code reference

All of this lives in **Step3Tree.vue**. Below is only the gecko: template, CSS, and the script data it uses.

---

## 1. Template (HTML)

```html
<div class="sheet-gecko-wrap">
  <div class="gecko gecko-portrait" :class="'gecko-tier-' + geckoTier">
    <!-- Connected figure: one body silhouette with overlapping parts -->
    <div class="gecko-figure">
      <span class="gecko-tail"></span>
      <span class="gecko-neck"></span>
      <span class="gecko-torso"></span>
      <span class="gecko-head">
        <span class="gecko-eye gecko-eye-l"></span>
        <span class="gecko-eye gecko-eye-r"></span>
        <span class="gecko-mouth"></span>
      </span>
      <span class="gecko-leg gecko-leg-l"></span>
      <span class="gecko-leg gecko-leg-r"></span>
    </div>
    <!-- Tier clothes (by item count) -->
    <span class="gecko-clothes gecko-hat" title="Hat (1+ items)"></span>
    <span class="gecko-clothes gecko-scarf" title="Scarf (21+)"></span>
    <span class="gecko-clothes gecko-vest" title="Vest (81+)"></span>
    <span class="gecko-clothes gecko-boots" title="Full outfit (201+)"></span>
    <!-- Group add-ons: one per selected group -->
    <span v-if="geckoAddons.essentials" class="gecko-addon gecko-underwear" title="Essentials"></span>
    <span v-if="geckoAddons.monitoring" class="gecko-addon gecko-glasses" title="Monitoring"></span>
    <span v-if="geckoAddons.logging" class="gecko-addon gecko-phone" title="Logging"></span>
    <span v-if="geckoAddons.backup" class="gecko-addon gecko-key" title="Backup & Restore"></span>
    <span v-if="geckoAddons.appCollection" class="gecko-addon gecko-crown" title="App Collection"></span>
    <!-- Flag in hand: Rancher version, class, versions -->
    <div class="gecko-flag">
      <span class="gecko-flag-pole"></span>
      <div class="gecko-flag-banner">
        <span class="gecko-flag-line">{{ flagLines.rancher }}</span>
        <span class="gecko-flag-line">{{ flagLines.class }}</span>
        <span class="gecko-flag-line">{{ flagLines.versions }}</span>
      </div>
    </div>
  </div>
</div>
```

---

## 2. Script (data the gecko uses)

```ts
// Gecko outfit tier from total items (charts + images): more selected = more clothed
const geckoTier = computed(() => {
  const total = sheetStats.value.charts + sheetStats.value.images
  if (total === 0) return 0
  if (total <= 20) return 1
  if (total <= 80) return 2
  if (total <= 200) return 3
  return 4
})

// Which group nodes are selected (walk full tree) for gecko add-ons
const selectedGroupIds = computed(() => {
  const ids = new Set<string>()
  function walk(nodes: TreeNode[] | undefined) {
    if (!nodes) return
    for (const n of nodes) {
      if (selected[n.id]) ids.add(n.id)
      walk(n.children)
    }
  }
  walk(props.roots)
  return ids
})
const geckoAddons = computed(() => ({
  essentials: selectedGroupIds.value.has('basic'),
  monitoring: selectedGroupIds.value.has('addon_monitoring'),
  logging: selectedGroupIds.value.has('addon_logging'),
  backup: selectedGroupIds.value.has('addon_backup-restore'),
  appCollection: selectedGroupIds.value.has('app_collection'),
}))

// Flag text: Rancher version, class, and distro versions
const flagLines = computed(() => {
  const rv = props.rancherVersion || '—'
  const classStr = sheetClass.value
  const parts: string[] = []
  if (props.k3sVersions && props.components.includes('k3s')) parts.push('K3s: ' + (props.k3sVersions.length > 12 ? props.k3sVersions.slice(0, 10) + '…' : props.k3sVersions))
  if (props.rke2Versions && props.components.includes('rke2')) parts.push('RKE2: ' + (props.rke2Versions.length > 12 ? props.rke2Versions.slice(0, 10) + '…' : props.rke2Versions))
  if (props.rkeVersions && props.components.includes('rke')) parts.push('RKE1: ' + (props.rkeVersions.length > 12 ? props.rkeVersions.slice(0, 10) + '…' : props.rkeVersions))
  const verStr = parts.length ? parts.join(' ') : '—'
  return { rancher: rv, class: classStr, versions: verStr }
})
```

*(Depends on: `props.roots`, `props.rancherVersion`, `props.components`, `props.k3sVersions`, `props.rke2Versions`, `props.rkeVersions`, `selected`, `sheetClass`, `sheetStats`.)*

---

## 3. CSS (all gecko styles)

```css
.sheet-gecko-wrap {
  flex-shrink: 0;
  margin-left: auto;
}
/* RPG character selection: connected figure, one body */
.gecko.gecko-portrait {
  position: relative;
  width: 140px;
  height: 170px;
}
.gecko-figure {
  position: absolute;
  inset: 0;
}
/* Tail connects to torso */
.gecko-tail {
  position: absolute;
  right: -4px;
  top: 70px;
  width: 38px;
  height: 16px;
  background: linear-gradient(90deg, #5a7a3a 0%, #6b8e23 100%);
  border-radius: 0 10px 10px 0;
  z-index: 0;
}
/* Neck connects head to torso */
.gecko-neck {
  position: absolute;
  left: 44px;
  top: 52px;
  width: 40px;
  height: 18px;
  background: linear-gradient(180deg, #6b8e23 0%, #5a7a3a 100%);
  border-radius: 0 0 8px 8px;
  z-index: 1;
}
.gecko-torso {
  position: absolute;
  left: 36px;
  top: 66px;
  width: 56px;
  height: 58px;
  background: linear-gradient(180deg, #7ba05b 0%, #5a7a3a 45%, #4a6a2a 100%);
  border-radius: 14px 14px 18px 18px;
  box-shadow: inset 0 2px 0 rgba(255,255,255,0.12);
  z-index: 1;
}
/* Head overlaps neck so figure is connected */
.gecko-head {
  position: absolute;
  left: 30px;
  top: 4px;
  width: 68px;
  height: 56px;
  background: linear-gradient(145deg, #6b8e23 0%, #556b2f 100%);
  border-radius: 50% 50% 48% 48%;
  box-shadow: inset 0 2px 0 rgba(255,255,255,0.2);
  z-index: 2;
}
.gecko-eye {
  position: absolute;
  top: 16px;
  width: 14px;
  height: 16px;
  background: #1a1a1a;
  border-radius: 50%;
  border: 2px solid #2d2d2d;
  box-shadow: inset 0 0 0 3px #fff;
}
.gecko-eye-l { left: 14px; }
.gecko-eye-r { right: 14px; left: auto; }
.gecko-mouth {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  width: 22px;
  height: 6px;
  border-bottom: 3px solid #3d3d1a;
  border-radius: 0 0 50% 50%;
}
/* Legs attached to torso */
.gecko-leg {
  position: absolute;
  bottom: 0;
  width: 20px;
  height: 30px;
  background: linear-gradient(180deg, #6b8e23 0%, #5a7a3a 40%, #4a6a2a 100%);
  border-radius: 10px 10px 0 0;
  z-index: 1;
}
.gecko-leg-l { left: 40px; }
.gecko-leg-r { right: 40px; left: auto; }
.gecko-clothes {
  position: absolute;
  opacity: 0;
  transition: opacity 0.35s ease;
  z-index: 3;
}
.gecko-tier-1 .gecko-hat,
.gecko-tier-2 .gecko-hat,
.gecko-tier-3 .gecko-hat,
.gecko-tier-4 .gecko-hat { opacity: 1; }
.gecko-tier-2 .gecko-scarf,
.gecko-tier-3 .gecko-scarf,
.gecko-tier-4 .gecko-scarf { opacity: 1; }
.gecko-tier-3 .gecko-vest,
.gecko-tier-4 .gecko-vest { opacity: 1; }
.gecko-tier-4 .gecko-boots { opacity: 1; }
.gecko-hat {
  left: 22px;
  top: -6px;
  width: 56px;
  height: 30px;
  background: linear-gradient(180deg, #8b4513 0%, #654321 100%);
  border-radius: 50% 50% 42% 42%;
  box-shadow: 0 4px 0 rgba(0,0,0,0.3);
}
.gecko-scarf {
  left: 42px;
  top: 38px;
  width: 56px;
  height: 24px;
  background: linear-gradient(90deg, #c41e3a, #8b0000);
  border-radius: 8px;
}
.gecko-vest {
  left: 34px;
  top: 56px;
  width: 72px;
  height: 54px;
  background: linear-gradient(180deg, #2f4f4f 0%, #1a2f2f 100%);
  border-radius: 12px 12px 20px 20px;
  border: 3px solid #3d5c5c;
}
.gecko-boots {
  left: 36px;
  bottom: -2px;
  width: 68px;
  height: 18px;
  background: linear-gradient(180deg, #1a1a1a 0%, #333 100%);
  border-radius: 8px 8px 0 0;
  z-index: 2;
}
/* Group add-ons (Essentials, Monitoring, Logging, Backup, App Collection) */
.gecko-addon {
  position: absolute;
  z-index: 4;
  opacity: 0;
  transition: opacity 0.3s ease;
}
.gecko-addon { opacity: 1; }
.gecko-underwear {
  left: 38px;
  top: 78px;
  width: 52px;
  height: 36px;
  background: linear-gradient(180deg, #4a4a6a 0%, #3a3a5a 100%);
  border-radius: 8px 8px 4px 4px;
  border: 2px solid #5a5a7a;
}
.gecko-glasses {
  left: 26px;
  top: 18px;
  width: 56px;
  height: 20px;
  border: 3px solid #2d2d2d;
  border-radius: 50%;
  background: transparent;
  box-shadow: inset 0 0 0 2px #555;
}
.gecko-glasses::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  width: 2px;
  height: 10px;
  background: #2d2d2d;
  transform: translateX(-50%);
}
.gecko-phone {
  right: -8px;
  bottom: 50px;
  left: auto;
  width: 18px;
  height: 32px;
  background: linear-gradient(180deg, #1a1a1a 0%, #333 100%);
  border-radius: 4px;
  border: 2px solid #444;
  box-shadow: 0 0 0 1px #222;
}
.gecko-phone::before {
  content: '';
  position: absolute;
  top: 4px;
  left: 50%;
  transform: translateX(-50%);
  width: 8px;
  height: 6px;
  background: #0a5;
  border-radius: 2px;
}
.gecko-key {
  left: -4px;
  bottom: 58px;
  width: 20px;
  height: 24px;
  background: linear-gradient(135deg, #8b7355 0%, #654321 100%);
  border-radius: 2px;
  clip-path: polygon(30% 0%, 70% 0%, 70% 100%, 30% 100%, 30% 60%, 0% 60%, 0% 40%, 30% 40%);
}
.gecko-crown {
  left: 28px;
  top: -14px;
  width: 56px;
  height: 28px;
  background: linear-gradient(180deg, #c9a227 0%, #8b6914 100%);
  clip-path: polygon(0% 100%, 10% 40%, 25% 60%, 50% 20%, 75% 60%, 90% 40%, 100% 100%);
  box-shadow: 0 2px 0 rgba(0,0,0,0.3);
}
/* Flag in hand: Rancher version, class, versions */
.gecko-flag {
  position: absolute;
  left: -2px;
  top: 72px;
  z-index: 5;
}
.gecko-flag-pole {
  position: absolute;
  left: 0;
  top: 0;
  width: 4px;
  height: 72px;
  background: linear-gradient(180deg, #654321 0%, #4a3210 100%);
  border-radius: 2px;
}
.gecko-flag-banner {
  position: absolute;
  left: 4px;
  top: 0;
  min-width: 88px;
  padding: 4px 6px;
  background: linear-gradient(135deg, #1a3a5a 0%, #0d1f33 100%);
  border: 1px solid #2a5a8a;
  border-radius: 2px 4px 4px 2px;
  font-size: 0.6rem;
  line-height: 1.25;
  color: #b8d4f0;
  box-shadow: 0 1px 3px rgba(0,0,0,0.4);
}
.gecko-flag-line {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 90px;
}
```

---

## Structure overview

| Part | Role |
|------|------|
| **.sheet-gecko-wrap** | Container on the right of the character sheet |
| **.gecko-figure** | Body: tail, neck, torso, head (eyes, mouth), legs |
| **.gecko-clothes** | Hat, scarf, vest, boots (visibility by `gecko-tier-*`) |
| **.gecko-addon** | Underwear, glasses, phone, key, crown (by selected groups) |
| **.gecko-flag** | Pole + banner with `flagLines.rancher`, `.class`, `.versions` |

File in repo: **`frontend/src/components/Step3Tree.vue`** (template ~lines 611–647, script ~272–313, styles ~809–1064).
