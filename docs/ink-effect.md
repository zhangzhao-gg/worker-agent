# 黄皮纸 + 不规则边缘 + 油墨字体效果实现

## 整体视觉架构

```
┌─────────────────────────────────────────────────┐
│  暗角 (vignette) — fixed, z-index: 9999         │
│  ┌─────────────────────────────────────────────┐ │
│  │  水渍色斑 (body::after) — absolute, z:9991  │ │
│  │  ┌─────────────────────────────────────────┐ │ │
│  │  │  颗粒肌理 (body::before) — abs, z:9990 │ │ │
│  │  │  ┌─────────────────────────────────────┐ │ │ │
│  │  │  │  页面内容 (正常文档流)              │ │ │ │
│  │  │  │    ├─ 左栏：正常渲染               │ │ │ │
│  │  │  │    └─ 右栏：黄皮纸纸带             │ │ │ │
│  │  │  └─────────────────────────────────────┘ │ │ │
│  │  └─────────────────────────────────────────┘ │ │
│  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

---

## 一、黄皮纸纸带

### 视觉目标

内容区域看起来像一条老旧的羊皮纸/电报纸带——泛黄底色、带横线纹理、有内阴影模拟纸张凹陷感。

### 核心 CSS

```css
.logList {
  background: #d4c4a8;
  background-image: linear-gradient(rgba(0,0,0,0.04) 1px, transparent 1px);
  background-size: 100% 1.6rem;
  box-shadow: inset 0 0 20px rgba(0,0,0,0.08);
  padding: 1.2rem 1.5rem 1.2rem 2rem;
  filter: url(#paper-edge);
}
```

### 逐行拆解

| 属性 | 作用 |
|------|------|
| `background: #d4c4a8` | 泛黄的羊皮纸底色，不是纯白也不是纯棕 |
| `background-image: linear-gradient(...)` | 用 1px 的半透明黑线 + 1.6rem 间距模拟横格纸纹 |
| `background-size: 100% 1.6rem` | 控制横线间距，与行高匹配 |
| `box-shadow: inset 0 0 20px rgba(0,0,0,0.08)` | 内阴影让纸张边缘有自然凹陷暗角 |
| `filter: url(#paper-edge)` | SVG 位移滤镜，让边缘不规则（见下节） |

---

## 二、不规则撕裂边缘

### 视觉目标

纸带不是规整矩形，而是有不规则的纤维撕裂边缘。阴影跟随不规则轮廓自然投射。

### HTML：SVG filter 定义（放在 body 内）

```html
<svg width="0" height="0" style="position:absolute">
  <filter id="paper-edge">
    <feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="4" result="noise"/>
    <feDisplacementMap in="SourceGraphic" in2="noise" scale="3"
      xChannelSelector="R" yChannelSelector="G"/>
  </filter>
</svg>
```

### CSS：应用 filter + 外层阴影

```css
/* 纸带本身 — 撕裂边缘 */
.logList {
  filter: url(#paper-edge);
}

/* 外层容器 — 阴影跟随不规则轮廓 */
.parchmentWrap {
  filter: drop-shadow(0 2px 3px rgba(26,20,16,0.2))
          drop-shadow(0 6px 12px rgba(26,20,16,0.1));
}
```

### 原理逐步拆解

#### 1. `feTurbulence` — 生成噪声位移图

```xml
<feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="4" result="noise"/>
```

- `type="fractalNoise"`：分形噪声，比 turbulence 更有机自然
- `baseFrequency="0.04"`：噪声基频。越小 → 波动越大越平缓（山丘感），越大 → 越密集细碎（砂纸感）
- `numOctaves="4"`：4 层不同频率的噪声倍频叠加。第 1 层大起伏，第 4 层小毛刺，合在一起模拟真实纸纤维断裂
- `result="noise"`：命名输出供下一步引用

#### 2. `feDisplacementMap` — 用噪声扭曲元素

```xml
<feDisplacementMap in="SourceGraphic" in2="noise" scale="3"
  xChannelSelector="R" yChannelSelector="G"/>
```

- `in="SourceGraphic"`：输入是原始 HTML 元素
- `in2="noise"`：用噪声纹理作为位移参考
- `scale="3"`：最大位移 3px。决定撕裂程度——值越大边缘越狂野
- `xChannelSelector="R"` / `yChannelSelector="G"`：红通道控制水平位移，绿通道控制垂直位移

**关键副作用**：displacement 不只作用于边缘——它扭曲整个元素的每个像素，包括文字。文字也会有微微不规则变形，像印在粗糙纸面上的活字印刷效果。本意做边缘，顺带给文字增加了真实印刷质感。

#### 3. `drop-shadow` vs `box-shadow`

```css
/* drop-shadow 跟随元素实际轮廓（包括 filter 扭曲后的形状） */
filter: drop-shadow(0 2px 3px rgba(26,20,16,0.2));

/* box-shadow 永远是矩形，无视 filter 变形 */
box-shadow: 0 2px 3px rgba(26,20,16,0.2);
```

`drop-shadow` 放在外层 `.parchmentWrap` 而非纸带本身，因为一个元素只能有一个 `filter` 属性。纸带的 filter 已经被 `url(#paper-edge)` 占用。

### 参数调节

| 效果 | 调整 |
|------|------|
| 更剧烈撕裂 | `scale` → 5-8 |
| 更细碎毛边 | `baseFrequency` → 0.08 |
| 更平滑波浪 | `baseFrequency` → 0.02，`numOctaves` → 2 |
| 更重阴影 | 增大 drop-shadow 模糊值和透明度 |

---

## 三、油墨文字效果

### 视觉目标

在黄皮纸上模拟钢笔/活字印刷的墨水渗透感——字迹边缘微微晕开，浓郁饱满，像墨水渗入纤维纸张。

### 核心 CSS

```css
.logContent {
  color: #1a1208;
  text-shadow: 0 0 0.5px #1a1208, 0.3px 0.3px 0.5px rgba(26,18,8,0.4);
  -webkit-font-smoothing: antialiased;
}
```

### 逐行拆解

#### 1. 颜色 `#1a1208`

不用纯黑 `#000`，而是极深暖棕色。真实墨水在老纸上从来不是纯黑的，而是带棕调的深色。文字和纸张在色温上统一。

#### 2. 第一层 text-shadow `0 0 0.5px #1a1208`

- 偏移：0, 0（不位移，均匀扩散）
- 模糊半径：0.5px
- 颜色：与文字同色

效果：笔画向四周均匀晕开约半个像素，模拟墨水被纸纤维吸收后的微扩散。这是油墨感的核心——笔画边缘不再锐利，有「浸润」的柔软度。

#### 3. 第二层 text-shadow `0.3px 0.3px 0.5px rgba(26,18,8,0.4)`

- 偏移：右下 0.3px（极微小）
- 模糊半径：0.5px
- 颜色：同色 40% 透明度

效果：模拟墨水因重力/纤维方向的轻微不均匀渗透。真实纸面墨迹不会完美对称，总有一侧略深。0.3px 肉眼几乎不可见，但潜意识感受到「这不是数字渲染的字」。

#### 4. `-webkit-font-smoothing: antialiased`

关闭浏览器默认亚像素渲染（会在笔画边缘引入 RGB 彩色条纹），改用灰度抗锯齿。边缘只有透明度变化，更接近真实印刷。

### 不适用场景

推理日志/代码类内容不应使用油墨效果。密集等宽字符需要锐利清晰的渲染，晕染会让阅读吃力。

### 参数调节

| 效果 | 调整 |
|------|------|
| 更浓的墨 | color → `#0f0a04` |
| 更明显渗透 | 第一层模糊 → `0 0 1px` |
| 更旧的感觉 | color 偏棕 → `#2a1f18`，降低对比度 |
| 更锐利新墨 | 模糊减到 `0.3px`，去掉第二层 |
| 打字机效果 | 去掉渗透，只保留偏移：`0.5px 0.5px 0 #1a1208` |

---

## 四、墙面做旧背景

### 组成层次

三个伪元素/元素叠加在页面内容之上（`pointer-events: none`），制造老墙质感：

#### 1. 颗粒肌理 `body::before`（z-index: 9990）

```css
body::before {
  content: '';
  position: absolute; inset: 0; min-height: 100%;
  z-index: 9990; pointer-events: none;
  background-image: url("data:image/svg+xml,...");
}
```

内联 SVG 生成 `feTurbulence` 分形噪声纹理，opacity 5%。模拟墙面水泥/石膏的微粒质感。

#### 2. 水渍色斑 `body::after`（z-index: 9991）

```css
body::after {
  content: '';
  position: absolute; inset: 0; min-height: 100%;
  z-index: 9991; pointer-events: none;
  background:
    radial-gradient(ellipse at 12% 15%, rgba(120,90,40,0.09) 0%, transparent 45%),
    radial-gradient(ellipse at 85% 75%, rgba(100,70,30,0.08) 0%, transparent 40%),
    radial-gradient(ellipse at 90% 10%, rgba(110,80,35,0.06) 0%, transparent 35%),
    radial-gradient(ellipse at 45% 85%, rgba(90,65,25,0.07) 0%, transparent 30%),
    radial-gradient(ellipse at 60% 40%, rgba(130,100,50,0.04) 0%, transparent 50%);
}
```

多个 `radial-gradient` 模拟岁月在墙面留下的水渍/茶渍痕迹，分散在不同位置。

#### 3. 电影暗角 `.vignette`（z-index: 9999，fixed）

```css
.vignette {
  position: fixed; inset: 0;
  pointer-events: none; z-index: 9999;
  background: radial-gradient(ellipse at center, transparent 55%, rgba(42,31,24,0.15) 100%);
}
```

四周渐暗，视觉聚焦到中央内容区。用 `fixed` 让它不跟随滚动。

### 为什么用 `position: absolute` 而非 `fixed`

肌理和水渍用 `absolute`——它们代表墙面本身的质感，应该跟随页面内容滚动，给人「这是一面真实的长墙」的感觉。

暗角用 `fixed`——它是摄影机/观察者视角的效果，永远框住当前视口。

---

## 五、字体策略

| 用途 | 字体 | 说明 |
|------|------|------|
| 英文标题/标签/元数据 | `Special Elite` | 本地 TTF，打字机风格 |
| 中文正文 | 系统字体栈 | `-apple-system, PingFang SC, Microsoft YaHei` |
| Fallback | `Menlo, Consolas, monospace` | Special Elite 缺字时的等宽回退 |

```css
@font-face {
  font-family: 'Special Elite';
  src: url('/fonts/SpecialElite.ttf') format('truetype');
  font-display: swap;
}

:root {
  --font: 'Special Elite', Menlo, Consolas, monospace;
  --font-body: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
```

英文元素（nav、label、badge、meta 时间戳）用 `var(--font)`，中文正文内容用 `var(--font-body)`。

---

## 六、完整配色

```css
:root {
  --ink: #2a1f18;          /* 深暖棕，主文字色 */
  --accent: #8b5e3c;       /* 中棕，强调/交互色 */
  --surface: #f0ebe4;      /* 浅暖灰，页面背景（老墙色） */
  --surface-alt: #e8e0d6;  /* 略深的备选背景 */
  --border: rgba(139,94,60,0.25);       /* 边框 */
  --border-light: rgba(139,94,60,0.12); /* 轻边框 */
  --muted: rgba(42,31,24,0.45);         /* 次要文字 */
}
```

黄皮纸纸带固定用 `#d4c4a8`，比页面背景更黄更深，形成「纸嵌在墙上」的层次。
