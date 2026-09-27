<!--
web/src/views/About.vue
页面：关于棋苑 · 版本与更新日志
职责：
- 展示楚汉棋苑平台定位、开源协议、GitHub 仓库入口与当前版本号
- 呈现严谨工业范式的全量历史演进更新日志流
-->

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()
const currentVersion = 'v0.0.11'
const releaseDate = '2026年9月27日'
</script>

<template>
  <div class="about-page">
    <!-- 头部品牌与版本信息 -->
    <div class="wood-panel about-header">
      <div class="brand-row">
        <div class="seal">楚</div>
        <div class="brand-info">
          <div class="title-line">
            <h2 class="font-brush">楚 汉 棋 苑</h2>
            <span class="version-tag font-brush">{{ currentVersion }} 最新版</span>
          </div>
          <p class="subtitle">传统国风双棋种在线对弈平台 · 单二进制自包含交付</p>
        </div>
      </div>

      <div class="meta-links">
        <div class="meta-item">
          <span class="m-label">开源协议：</span>
          <span class="m-val">MIT License</span>
        </div>
        <div class="meta-item">
          <span class="m-label">发布日期：</span>
          <span class="m-val">{{ releaseDate }}</span>
        </div>
        <div class="meta-item">
          <span class="m-label">开源仓库：</span>
          <a
            href="https://github.com/XIMOYA/Chu-Han-Chess-Court"
            target="_blank"
            class="repo-link"
          >
            GitHub / Chu-Han-Chess-Court ↗
          </a>
        </div>
        <el-button size="small" plain @click="router.push('/')">返回大厅</el-button>
      </div>
    </div>

    <!-- 更新日志卷轴 -->
    <div class="dark-panel changelog-container">
      <div class="cl-header">
        <h3 class="font-brush cl-main-title">版 本 演 进 史 卷</h3>
        <p class="cl-desc">全量记录楚汉棋苑自首发版本以来的全部新特性、核心修复、性能调优与架构治理轨迹。</p>
      </div>

      <div class="changelog-stream">
        <!-- v0.0.11 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.11</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>对弈大厅新增【江湖擂台 · 待弈榜】公开大榜，实时呈现全服公开等待中的对弈房间，支持其他在线棋友一键应擂入局。</li>
              <li>新增一键【极速随缘匹配】撮合算法，毫秒级扫描全服公开空闲擂台实现秒级配对；若暂无空闲擂台则自动设擂候弈。</li>
              <li>摆擂创建房间新增“入局准入”选项，支持在“公开招擂”与“仅凭房号”两种可见度模式之间自由切换。</li>
            </ul>
          </div>
          <div class="re-section">
            <h5 class="sec-label">修复</h5>
            <ul>
              <li>修复残局关卡列表在侧边栏由于容器高度未加约束导致纵向滚动条失效、第 11 至 30 关被截断无法向下翻看的问题。</li>
              <li>修复残局页面侧边栏 Tab 标签贴近古木卡片左边框、内边距缺失造成的视觉压迫感，补充柔和呼吸留白。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.10 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.10</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>题库规模扩充至整整 30 道传世经典象棋残局，涵盖《适情雅趣》《橘中秘》《百局象棋谱》《韬略元机》《心武残编》《渊深海阔》等九大典籍名谱。</li>
              <li>为题库初始化引入按题目雅号查重的无感平滑写入机制，存量数据库升级时自动补充新增名谱，且保留历史打卡数据。</li>
            </ul>
          </div>
          <div class="re-section">
            <h5 class="sec-label">质量与测试</h5>
            <ul>
              <li>新增全量 30 道古谱 FEN 局面串与九宫/过河边界的自动化单元测试回归套件，确保数据结构 100% 合规。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.9 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.9</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>新增“经典残局闯关 · 每日一题”系统，基于每日日期哈希自动轮换今日名局，全服棋友同解一局。</li>
              <li>实现红先连照绝杀推演算法与黑方智能应手机制；破局达成瞬间印下古风朱砂印章【破局成功 · 参透玄机】并记录连续打卡天数。</li>
              <li>管理员后台新增第 6 大板块“经史子集 · 残局题库”，支持关卡序数、标题、FEN 局面与锦囊的可视化修缮与录入。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.8 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.8</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>系统设置新增“神机妙算 · 大语言模型（LLM）配置中心”，全面支持 DeepSeek、通义千问、Kimi、OpenAI 及本地 Ollama 等标准协议节点。</li>
              <li>支持在线一键安全拉取当前 Key 授权的全部可用模型清单，并支持下拉框动态搜索与自定义手输。</li>
              <li>新增单轮模型对话实测功能，提供毫秒级响应延时测量与真实回答内容诊断。</li>
            </ul>
          </div>
          <div class="re-section">
            <h5 class="sec-label">性能与计量</h5>
            <ul>
              <li>新增大模型全平台请求频次与 Token 消耗用量统计看板，支持 Prompt/Completion 分项用量追踪及管理员一键清零。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.7 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.7</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>新增纯 Go 原生智能人机博弈（PvE）系统，基于极小化极大算法、Alpha-Beta 剪枝与 PST 权重矩阵驱动象棋与五子棋双 AI。</li>
              <li>提供初出茅庐（初学）、略有小成（进阶）、炉火纯青（大师）三大国风棋力品阶。</li>
              <li>为人机对战引入 450ms ~ 900ms 拟真思考延时；对局中点击悔棋由系统自动秒级同意并瞬退两步，实现零等待试招推演。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.6 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.6</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>账号注册全面支持邮箱验证码核销体系；构建原生 Go SMTP 邮件发信引擎，支持 SSL/TLS 465 端口发信与 60 秒防刷限流。</li>
              <li>管理员后台新增“棋苑通规 · 系统设置”专区，支持动态控制是否开放注册、是否强制验证码核销以及最低密码长度限制。</li>
              <li>后台新增 SMTP 邮件隧道一键连通性测试诊断功能，并内置典雅古风 HTML 验证信模板。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.5 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.5</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>新增对局交互式复盘与回放系统（/replay/:id），支持开局/上步/播放/下步/终局播放控制台、无级时间轴进度拖拽、键盘快捷键与走法高亮双向联动。</li>
              <li>大厅最近战报列表与管理员档案名册均配备一键直达复盘入口，支持免登录外链分享。</li>
            </ul>
          </div>
          <div class="re-section">
            <h5 class="sec-label">修复</h5>
            <ul>
              <li>修复管理员在后台查阅棋友名册时，SQLite 在 MaxOpenConns(1) 下游标未释放导致全局连接池发生死锁并牵连全站卡死的问题。</li>
              <li>修复对局室 Element Plus Tab 内部容器高度丢失导致聊天区域羊皮纸背景无法撑满右侧面板的布局塌陷缺陷。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.4 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.4</h4>
            <time class="re-date">2026年9月27日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>新增基于 Web Audio API 的原生物理建模对弈音效系统，实时合成实木叩盘落子声、双木相撞吃子声、金石弦鸣将军声与大三和弦胜利之声。</li>
              <li>对局室提供随手可按的【音效开 / 静音】切换功能，偏好状态持久化保存在本地。</li>
            </ul>
          </div>
          <div class="re-section">
            <h5 class="sec-label">改进</h5>
            <ul>
              <li>重构棋盘缩放算法为横纵双向等比自适应约束，彻底消除大窗口下的垂直滚动条，保障对弈核心区域全屏一览无余。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.3 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.3</h4>
            <time class="re-date">2026年9月26日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">修复</h5>
            <ul>
              <li>修复五子棋前端坐标键名 (row/col) 与后端协议结构体标签 (r/c) 不匹配导致落子坐标全部坍缩为默认零值 (0,0) 的重大数据反序列化缺陷。</li>
              <li>为前后端坐标模型加入智能双向兼容解析器，彻底根除坐标丢失。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.2 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.2</h4>
            <time class="re-date">2026年9月26日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>新增五子棋（Gomoku）黑白对弈玩法，实现 15×15 经纬格线网格、3D 立体拟真黑曜石与羊脂玉双色棋子、落子半透明预瞄圈与最后一手红点标记。</li>
              <li>大厅支持选择双棋种开局与战报归档，棋手卡片自适应呈现黑白专属徽章。</li>
            </ul>
          </div>
        </article>

        <!-- v0.0.1 -->
        <article class="release-entry">
          <header class="re-header">
            <h4 class="re-version">v0.0.1</h4>
            <time class="re-date">2026年9月25日</time>
          </header>
          <div class="re-section">
            <h5 class="sec-label">新功能</h5>
            <ul>
              <li>楚汉棋苑首发开源版本上线；Go 原生标准库 + Vue 3 Canvas + 纯 Go SQLite 构建中国象棋在线对弈平台。</li>
              <li>完整实现中国象棋国家标准赛制规则：蹩马腿、塞象眼、九宫限制、将帅不照面判定；将军检测、将死/困毙判定、长将长捉判负；自动生成国家标准中文棋谱。</li>
              <li>支持好友双人联机房间制、好友凭号加入、悔棋、认输、求和与局内实时聊天。</li>
              <li>前端全量静态资源通过 go:embed 完整嵌入单一可执行文件，开箱即用。</li>
            </ul>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>

<style scoped>
.about-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
}

.about-header {
  padding: 20px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 14px;
}

.seal {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  font-size: 30px;
  color: #f5e4c0;
  background: linear-gradient(145deg, #b23a26, #8c2517);
  border: 2px solid #d8b36a;
  border-radius: 8px;
  font-family: 'Ma Shan Zheng', cursive;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.45);
}

.title-line {
  display: flex;
  align-items: center;
  gap: 10px;
}

.title-line h2 {
  margin: 0;
  font-size: 26px;
  color: #7a2415;
  letter-spacing: 3px;
}

.version-tag {
  background: #a83323;
  color: #f5e4c0;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid #d8b36a;
  letter-spacing: 1px;
}

.subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #8c6a38;
}

.meta-links {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.meta-item {
  font-size: 13px;
  color: #6b4a24;
}

.m-val {
  font-weight: 700;
  color: #432a12;
}

.repo-link {
  color: #a83323;
  font-weight: 700;
  text-decoration: none;
}

.repo-link:hover {
  text-decoration: underline;
}

.changelog-container {
  padding: 22px 28px;
}

.cl-header {
  border-bottom: 1px solid rgba(201, 162, 75, 0.35);
  padding-bottom: 14px;
  margin-bottom: 20px;
}

.cl-main-title {
  margin: 0;
  font-size: 24px;
  color: #f0d89a;
  letter-spacing: 3px;
}

.cl-desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: #b9a171;
}

.changelog-stream {
  display: flex;
  flex-direction: column;
  gap: 26px;
}

.release-entry {
  border-bottom: 1px dashed rgba(201, 162, 75, 0.25);
  padding-bottom: 22px;
}

.release-entry:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.re-header {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 10px;
}

.re-version {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: #f0d89a;
  letter-spacing: 1px;
  font-family: monospace;
}

.re-date {
  font-size: 13px;
  color: #a88a5a;
}

.re-section {
  margin-top: 10px;
}

.sec-label {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 700;
  color: #d8c39a;
}

ul {
  margin: 0;
  padding-left: 20px;
}

li {
  font-size: 13.5px;
  line-height: 1.75;
  color: #efe0c0;
  margin-bottom: 4px;
}
</style>
