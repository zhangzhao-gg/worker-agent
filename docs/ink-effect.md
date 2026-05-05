# 油墨效果实现原理

## 视觉目标

在羊皮纸底色（#d4c4a8）上模拟钢笔/活字印刷的墨水渗透感——字迹边缘微微晕开，颜色浓郁饱满，像真正的墨水渗入纤维纸张。

## 核心 CSS

```css
.ink-text {
  color: #1a1208;
  text-shadow: 0 0 0.5px #1a1208, 0.3px 0.3px 0.5px rgba(26,18,8,0.4);
  -webkit-font-smoothing: antialiased;
}
```

## 逐行拆解

### 1. 颜色选择 `#1a1208`

不用纯黑 `#000`，而是极深的暖棕色。真实墨水在老纸上从来不是纯黑的，而是带有棕调的深色。这让文字和纸张在色温上统一。

### 2. 第一层 text-shadow `0 0 0.5px #1a1208`

- 偏移：0, 0（不位移，均匀扩散）
- 模糊半径：0.5px
- 颜色：与文字同色

效果：文字笔画向四周均匀晕开约半个像素，模拟墨水被纸张纤维吸收后的微扩散。这是油墨感的核心——让笔画边缘不再锐利，而是有一种「浸润」的柔软度。

### 3. 第二层 text-shadow `0.3px 0.3px 0.5px rgba(26,18,8,0.4)`

- 偏移：右下 0.3px（极微小）
- 模糊半径：0.5px
- 颜色：同色但 40% 透明度

效果：模拟墨水在纸面因重力/纤维方向产生的轻微不均匀渗透。真实纸张上墨迹不会是完美对称的，总有一侧略深。0.3px 的偏移肉眼几乎不可见，但潜意识能感受到「这不是数字渲染的字」。

### 4. `-webkit-font-smoothing: antialiased`

关闭浏览器默认的亚像素渲染（subpixel-antialiased），改用灰度抗锯齿。亚像素渲染会在笔画边缘引入彩色条纹（RGB 子像素），破坏油墨的单色浓郁感。灰度抗锯齿让边缘只有透明度变化，更接近真实印刷。

## 为什么不适合推理日志

推理日志是代码/结构化数据，需要清晰锐利的等宽字体阅读体验。油墨的晕染会让密集的代码文本变得模糊吃力。油墨效果适合叙事性的段落文字，不适合需要精确辨认每个字符的场景。

## 参数调节指南

| 想要的效果 | 调整方式 |
|-----------|---------|
| 更浓的墨 | 加深 color，如 `#0f0a04` |
| 更明显的渗透 | 增大第一层模糊，如 `0 0 1px` |
| 更旧的感觉 | color 偏棕，如 `#2a1f18`，降低对比度 |
| 更锐利（新墨） | 减小模糊到 `0.3px`，去掉第二层 |
| 打字机效果 | 去掉渗透，只保留偏移：`0.5px 0.5px 0 #1a1208` |

---

## 撕裂纸张边缘效果

### 视觉目标

纸带不是规整的矩形，而是有不规则的纤维撕裂边缘，像真正从卷筒上撕下来的纸。阴影跟随不规则轮廓自然投射。

### 核心实现

```html
<!-- HTML: 内联 SVG filter 定义 -->
<svg width="0" height="0" style="position:absolute">
  <filter id="paper-edge">
    <feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="4" result="noise"/>
    <feDisplacementMap in="SourceGraphic" in2="noise" scale="3" xChannelSelector="R" yChannelSelector="G"/>
  </filter>
</svg>
```

```css
/* CSS: 应用到纸带元素 */
.logList {
  filter: url(#paper-edge);
}

/* 阴影在外层容器，跟随不规则轮廓 */
.parchmentWrap {
  filter: drop-shadow(0 2px 3px rgba(26,20,16,0.2))
          drop-shadow(0 6px 12px rgba(26,20,16,0.1));
}
```

### 逐步拆解

#### 1. `feTurbulence` — 生成噪声纹理

```xml
<feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="4" result="noise"/>
```

- `type="fractalNoise"`：分形噪声，比普通 turbulence 更有机、更像自然纹理
- `baseFrequency="0.04"`：噪声的基础频率。越小波动越大越平缓（像山丘），越大越密集细碎（像砂纸）。0.04 产生适合纸张边缘的中等波动
- `numOctaves="4"`：叠加 4 层不同频率的噪声（倍频叠加）。层数越多细节越丰富——第 1 层是大的起伏，第 4 层是小的毛刺，合在一起就像真实纸张纤维的断裂
- `result="noise"`：输出命名为 "noise"，供下一步引用

#### 2. `feDisplacementMap` — 用噪声扭曲图形

```xml
<feDisplacementMap in="SourceGraphic" in2="noise" scale="3" xChannelSelector="R" yChannelSelector="G"/>
```

- `in="SourceGraphic"`：输入是原始元素（纸带）
- `in2="noise"`：用上一步生成的噪声作为位移图
- `scale="3"`：最大位移 3 像素。这决定了边缘的「撕裂程度」。值越大越狂野，3px 刚好是细微但可见的不规则
- `xChannelSelector="R"` / `yChannelSelector="G"`：用噪声的红色通道控制水平位移，绿色通道控制垂直位移

**关键洞察**：这个 filter 不只作用于边缘——它扭曲整个元素的每一个像素，包括文字。所以文字也会有微微的不规则变形，看起来像是印在粗糙纸面上墨水沿纤维方向的轻微扩散和错位。这是一个"意外之喜"——本意是做边缘，但顺带给文字增加了真实的印刷质感。

#### 3. `drop-shadow` — 阴影跟随轮廓

```css
filter: drop-shadow(0 2px 3px rgba(26,20,16,0.2))
        drop-shadow(0 6px 12px rgba(26,20,16,0.1));
```

和 `box-shadow` 不同，`drop-shadow` 跟随元素的实际轮廓（包括经过 filter 扭曲后的不规则形状）。所以阴影也是不规则的，看起来就像一张撕裂的纸放在桌面上的自然投影。

注意：`drop-shadow` 放在外层 `.parchmentWrap` 上而不是纸带本身上，因为纸带自己已经有 `filter: url(#paper-edge)`，CSS 中一个元素只能有一个 filter 属性。

### 参数调节

| 想要的效果 | 调整方式 |
|-----------|---------|
| 更剧烈的撕裂 | 增大 `scale`，如 5-8 |
| 更细碎的毛边 | 增大 `baseFrequency`，如 0.08 |
| 更平滑的波浪 | 减小 `baseFrequency`，如 0.02，减少 `numOctaves` |
| 只要边缘不要影响文字 | 不能——displacement 是全局的。如果要保护文字，需要把纸带背景和文字内容分层 |
| 更重的阴影 | 增大 drop-shadow 的模糊和透明度 |
