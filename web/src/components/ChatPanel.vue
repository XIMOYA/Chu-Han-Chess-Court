<!--
web/src/components/ChatPanel.vue
组件：房间局内实时聊天面板
职责：
- 展示历史聊天消息列表并支持自动滚动至最新
- 提供消息输入框与回车发送，过滤中文输入法（IME）合成状态
- 针对游客实施禁言保护，引导登录后参与互动
-->

<script setup>
import { nextTick, ref, watch } from 'vue'

const props = defineProps({
  chats: { type: Array, default: () => [] },
  canSend: { type: Boolean, default: false }
})

const emit = defineEmits(['send'])
const text = ref('')
const body = ref(null)

function submit(e) {
  if (e?.isComposing) return
  const t = text.value.trim()
  if (!t) return
  emit('send', t)
  text.value = ''
}

watch(
  () => props.chats.length,
  async () => {
    await nextTick()
    if (body.value) body.value.scrollTop = body.value.scrollHeight
  }
)
</script>

<template>
  <div class="chat">
    <div ref="body" class="chat-body">
      <div v-for="(c, i) in chats" :key="i" class="chat-line">
        <span class="chat-name">{{ c.name }}：</span>
        <span class="chat-text">{{ c.text }}</span>
      </div>
      <div v-if="!chats.length" class="chat-empty">还没有消息，说点什么吧。</div>
    </div>
    <div v-if="canSend" class="chat-input">
      <el-input
        v-model="text"
        size="default"
        placeholder="输入消息，回车发送"
        maxlength="200"
        @keydown.enter="submit"
      />
      <el-button type="primary" @click="submit">发送</el-button>
    </div>
    <div v-else class="chat-ban">游客只能观战，登录后可参与聊天</div>
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: 100%;
  flex: 1;
  min-height: 0;
}

.chat-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chat-line {
  font-size: 14px;
  line-height: 1.7;
  word-break: break-all;
  animation: msgSlideIn 0.22s cubic-bezier(0.2, 0.8, 0.2, 1);
}

@keyframes msgSlideIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.chat-name {
  color: #a83323;
  font-weight: 600;
}

.chat-text {
  color: #3c2812;
}

.chat-empty {
  color: #9a7a4a;
  font-size: 14px;
  text-align: center;
  margin: auto;
  padding: 40px 0;
}

.chat-input {
  display: flex;
  gap: 8px;
  padding: 10px 12px;
  border-top: 1px solid rgba(120, 80, 40, 0.3);
  background: rgba(247, 235, 210, 0.98);
  border-radius: 0 0 10px 10px;
}

.chat-ban {
  padding: 12px;
  text-align: center;
  font-size: 12px;
  color: #8a6638;
  border-top: 1px solid rgba(120, 80, 40, 0.3);
  background: rgba(247, 235, 210, 0.98);
  border-radius: 0 0 10px 10px;
}
</style>
