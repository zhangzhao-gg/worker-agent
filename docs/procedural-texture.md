# 程序化纹理系统 — 噪点肌理 + 木桌质感

纯 CSS/SVG 生成，零图片依赖。核心手法：**SVG feTurbulence 做微观噪点** + **多层 CSS gradient 做宏观纹路**。

---

## 一、噪点肌理（肮脏黑点）

### 视觉目标

整个页面覆盖一层若隐若现的颗粒感，模拟老墙面水泥/石膏的微粒质感。肉眼看是均匀散布的极淡"脏点"。

### 实现位置

`src/index.css` — `body::before` 伪元素

### 核心代码

```css
body::before {
  content: '';
  position: absolute;
  inset: 0;
  min-height: 100%;
  z-index: 9990;
  pointer-events: none;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.75' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.05'/%3E%3C/svg%3E");
}
```

### 原理拆解

```
┌─────────────────────────────────────────────────────────┐
│  data:image/svg+xml (内联 SVG，浏览器直接渲染)           │
│                                                         │
│  <feTurbulence                                          │
│    type="fractalNoise"    ← 分形噪声（Perlin noise 变种）│
│    baseFrequency="0.75"   ← 颗粒密度：0.75 = 密集细腻   │
│    numOctaves="4"         ← 4 层频率叠加，增加细节       │
│    stitchTiles="stitch"   ← 平铺接缝无缝衔接             │
│  />                                                     │
│                                                         │
│  <rect opacity="0.05"/>   ← 5% 透明度，极度克制          │
└─────────────────────────────────────────────────────────┘
```

### 参数含义

| 参数 | 值 | 效果 | 调节方向 |
|------|----|------|----------|
| `type` | `fractalNoise` | 平滑有机的分形噪声 | 换 `turbulence` → 更粗暴水流感 |
| `baseFrequency` | `0.75` | 决定颗粒大小。越大 → 颗粒越小越密 | `0.3` = 大斑块，`1.5` = 极细砂纸 |
| `numOctaves` | `4` | 频率倍增叠加层数。越多 → 细节越丰富 | `1` = 平滑，`6` = 毛糙 |
| `stitchTiles` | `stitch` | 平铺时边缘无缝 | 必须保留，否则有明显接缝 |
| `opacity` | `0.05` | 噪点强度 | `0.02` = 几乎不可见，`0.15` = 明显脏 |

### 与水渍色斑的协同

`body::after`（z-index: 9991）叠加 5 个 `radial-gradient` 模拟不规则水渍：

```css
body::after {
  background:
    radial-gradient(ellipse at 12% 15%, rgba(120,90,40,0.09) ..., transparent 45%),
    radial-gradient(ellipse at 85% 75%, rgba(100,70,30,0.08) ..., transparent 40%),
    radial-gradient(ellipse at 90% 10%, rgba(110,80,35,0.06) ..., transparent 35%),
    radial-gradient(ellipse at 45% 85%, rgba(90,65,25,0.07) ..., transparent 30%),
    radial-gradient(ellipse at 60% 40%, rgba(130,100,50,0.04) ..., transparent 50%);
}
```

**两层叠加效果**：噪点提供均匀微观颗粒 + 色斑提供随机宏观暗区 = 真实老墙。

---

## 二、木桌质感

### 视觉目标

访客视图的 `.desk` 容器看起来像一张深色实木书桌——有木纹走向、年轮深沟、木节疤、表面颗粒感。

### 实现位置

`src/pages/WorkerDetail.module.css` — `.desk` 的 `background` 属性

### 分层架构

```
┌─ 第 1 层：粗木纹竖条（repeating-linear-gradient 92°）
├─ 第 2 层：年轮深沟（repeating-linear-gradient 86°）
├─ 第 3 层：木节疤 ×3（radial-gradient ellipse）
├─ 第 4 层：表面颗粒噪点（内联 SVG feTurbulence）
└─ 第 5 层：基色渐变（linear-gradient 175°）
    渲染顺序：第 1 层在最上面，第 5 层在最底部
```

### 逐层解析

#### 第 1 层 — 粗木纹竖条

```css
repeating-linear-gradient(
  92deg,                          /* 几乎垂直，微偏 2° → 木纹不死板 */
  transparent 0px,
  transparent 5px,                /* 5px 间隙 */
  rgba(0,0,0,0.07) 5px,
  rgba(0,0,0,0.07) 7px,          /* 2px 宽暗线 — 细木纹 */
  transparent 7px,
  transparent 14px,               /* 7px 间隙 */
  rgba(30,15,5,0.06) 14px,
  rgba(30,15,5,0.06) 16px         /* 2px 宽暖棕线 — 色彩变化 */
)
```

**设计意图**：两种颜色（纯黑透明 + 暖棕）交替出现，间距不均匀（5px、7px），模拟真实木材纤维的不规则密度。

#### 第 2 层 — 年轮深沟

```css
repeating-linear-gradient(
  86deg,                          /* 与第 1 层偏差 6°，制造交叉感 */
  transparent 0px,
  transparent 18px,
  rgba(0,0,0,0.12) 18px,         /* ← 最深的沟，12% 不透明 */
  rgba(0,0,0,0.12) 21px,         /* 3px 宽 */
  transparent 21px,
  transparent 45px,
  rgba(40,20,5,0.09) 45px,       /* 中等深度 */
  rgba(40,20,5,0.09) 49px,       /* 4px 宽 */
  transparent 49px,
  transparent 70px,
  rgba(0,0,0,0.07) 70px,         /* 最浅的纹 */
  rgba(0,0,0,0.07) 72px          /* 2px 宽 */
)
```

**设计意图**：三种不同深度（12%、9%、7%）+ 三种不同宽度（3px、4px、2px）+ 不等间距（18px、24px、25px），模拟年轮的自然疏密变化。

**关键技巧**：第 1 层 92° 与第 2 层 86° 的 **6° 角度差** 让两组线条产生微妙交叉，避免平行线的机械感，模拟木纹方向的自然偏移。

#### 第 3 层 — 木节疤

```css
radial-gradient(ellipse 30px 20px at 20% 35%, rgba(30,15,5,0.18) 0%, transparent 70%),
radial-gradient(ellipse 22px 16px at 75% 65%, rgba(25,12,3,0.14) 0%, transparent 65%),
radial-gradient(ellipse 18px 24px at 55% 15%, rgba(35,18,5,0.1) 0%, transparent 60%)
```

| 节疤 | 尺寸 | 位置 | 深度 |
|------|------|------|------|
| 大节疤 | 30×20px | 左上 (20%, 35%) | 18% 深棕 |
| 中节疤 | 22×16px | 右下 (75%, 65%) | 14% 暗棕 |
| 小节疤 | 18×24px | 中上 (55%, 15%) | 10% 浅棕 |

**设计意图**：椭圆形（非正圆）、不同朝向、不等大小 → 自然木节。三个够了，多了会假。

#### 第 4 层 — 表面颗粒噪点

```css
url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' ...%3E
  %3CfeTurbulence type='fractalNoise' baseFrequency='0.65' numOctaves='5' stitchTiles='stitch'/%3E
  %3Crect ... opacity='0.09'/%3E
%3C/svg%3E")
```

| 对比墙面噪点 | 墙面 (body::before) | 木桌 (第 4 层) |
|--------------|---------------------|----------------|
| baseFrequency | 0.75 (更密) | 0.65 (稍粗) |
| numOctaves | 4 | 5 (更多细节) |
| opacity | 0.05 (极淡) | 0.09 (更明显) |

木桌噪点比墙面更粗、更强，因为木材表面比水泥墙的颗粒感更明显。

#### 第 5 层 — 基色渐变

```css
linear-gradient(
  175deg,                    /* 几乎垂直，模拟光照方向 */
  #4a2e15 0%,               /* 深胡桃棕 */
  #5c3a1e 12%,              /* 稍亮 */
  #6b4423 30%,              /* 中棕高光区 */
  #573518 50%,              /* 回暗 */
  #6e4825 65%,              /* 再亮 */
  #5a3818 80%,              /* 暗 */
  #4a2a12 100%              /* 深棕收尾 */
)
```

**设计意图**：7 个色阶不是线性过渡，而是明暗交替（亮-暗-亮-暗），模拟木板表面被光线从一侧照射时的明暗起伏，以及不同年轮区域颜色的自然深浅差异。

### 边框与阴影

```css
.desk {
  border-radius: 6px;
  border: 3px solid #3d2410;      /* 深色实边 → 桌面厚度 */
  box-shadow:
    inset 0 1px 4px rgba(0,0,0,0.3),          /* 内阴影 → 凹陷质感 */
    inset 0 -1px 2px rgba(255,200,100,0.05),   /* 底部微暖光 → 反射 */
    0 4px 20px rgba(40,25,10,0.4),             /* 近距投影 → 桌面浮起 */
    0 12px 40px rgba(40,25,10,0.2);            /* 远距软影 → 空间深度 */
}
```

---

## 三、纸片做旧纹理

### 位置

`WorkerDetail.module.css` — `.paper` 类

### 分层

```css
.paper {
  background-color: var(--amb-paper, #d4c4a8);
  background-image:
    linear-gradient(rgba(0,0,0,0.03) 1px, transparent 1px),   /* 横线纹 */
    radial-gradient(ellipse ... at 85% 20%, ...),              /* 咖啡渍 ×3 */
    linear-gradient(155deg, ... 40%, ... 40.5%, ...),          /* 褶皱折痕 ×2 */
    radial-gradient(ellipse at center, transparent 50%, ...),  /* 边缘泛黄 */
    url("data:image/svg+xml,...");                             /* 纸纤维噪点 */
}
```

| 层 | 技术 | 模拟 |
|----|------|------|
| 横线纹 | 1px 半透明线 + 1.6rem 间距 | 稿纸横格 |
| 咖啡渍 | 3 个 radial-gradient 暖色椭圆 | 年久的茶渍/指印 |
| 褶皱 | 2 条极窄 linear-gradient（0.5% 宽） | 对折痕迹 |
| 泛黄 | 中心透明→边缘暖色 radial-gradient | 纸张边缘氧化 |
| 纤维噪点 | SVG feTurbulence (baseFrequency=1.2) | 纸浆纤维颗粒 |

---

## 四、设计哲学

### 为什么用程序化纹理而非图片

1. **零网络请求** — 所有纹理内联在 CSS 中，无需加载图片
2. **无限分辨率** — SVG 滤镜是实时计算的，任何缩放都锐利
3. **可调参数** — 改一个数字就能调风格，不用回 Photoshop
4. **体积极小** — 整个视觉系统不到 3KB CSS

### 核心技巧总结

| 技巧 | 用途 | 关键参数 |
|------|------|----------|
| `feTurbulence` + 内联 SVG | 微观随机噪点 | baseFrequency 控制粗细 |
| `repeating-linear-gradient` 多层 | 宏观线条纹路 | 角度差 + 不等间距 |
| `radial-gradient` ellipse | 局部暗斑/节疤 | 尺寸 + 位置 + 透明度 |
| 多层 `background` 叠加 | 复合质感 | 从精细→粗犷依次叠加 |
| 非整数角度差 (92° vs 86°) | 避免机械感 | 差值 4-8° 最自然 |
| 明暗交替渐变 | 光照模拟 | 非线性色阶分布 |
