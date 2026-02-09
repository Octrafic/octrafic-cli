<template>
  <div class="page-actions">
    <button @click="copyPage" class="action-btn" title="Copy page content">
      <Copy :size="16" />
      <span>Copy page</span>
    </button>

    <div class="ai-dropdown">
      <button @click="toggleDropdown" class="action-btn" title="Ask AI about this page">
        <Sparkles :size="16" />
        <span>Ask AI</span>
        <ChevronDown :size="12" class="chevron" />
      </button>

      <div v-if="showDropdown" class="dropdown-menu">
        <button @click="askChatGPT" class="dropdown-item chatgpt-item">
          <span>ChatGPT</span>
        </button>
        <button @click="askClaude" class="dropdown-item claude-item">
          <span>Claude</span>
        </button>
        <button @click="askPerplexity" class="dropdown-item perplexity-item">
          <span>Perplexity</span>
        </button>
      </div>
    </div>

    <div v-if="showNotification" class="notification">
      {{ notificationText }}
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useData } from 'vitepress'
import { Copy, Sparkles, ChevronDown } from 'lucide-vue-next'

const { page } = useData()
const showDropdown = ref(false)
const showNotification = ref(false)
const notificationText = ref('')

const toggleDropdown = () => {
  showDropdown.value = !showDropdown.value
}

const closeDropdown = (e) => {
  if (!e.target.closest('.ai-dropdown')) {
    showDropdown.value = false
  }
}

const notify = (text) => {
  notificationText.value = text
  showNotification.value = true
  setTimeout(() => {
    showNotification.value = false
  }, 2000)
}

const getPageContent = () => {
  const title = page.value.title
  // Get text content from the rendered page
  const docElement = document.querySelector('.vp-doc')
  const content = docElement ? docElement.innerText : ''
  return `# ${title}\n\n${content}`
}

const copyPage = () => {
  try {
    const content = getPageContent()
    const textArea = document.createElement('textarea')
    textArea.value = content
    textArea.style.position = 'fixed'
    textArea.style.opacity = '0'
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    notify('Page copied to clipboard')
  } catch (err) {
    console.error('Copy error:', err)
    notify('Failed to copy')
  }
}

const askChatGPT = () => {
  try {
    const pageUrl = window.location.href
    const prompt = `Please help me understand this documentation page:\n${pageUrl}\n\n`
    const url = `https://chatgpt.com/?prompt=${encodeURIComponent(prompt)}`
    window.open(url, '_blank')?.focus()
    notify('Opening ChatGPT...')
    showDropdown.value = false
  } catch (err) {
    console.error('ChatGPT error:', err)
    notify('Failed to open ChatGPT')
  }
}

const askClaude = () => {
  try {
    const pageUrl = window.location.href
    const prompt = `Please help me understand this documentation page:\n${pageUrl}\n\n`
    const url = `https://claude.ai/new?q=${encodeURIComponent(prompt)}`
    window.open(url, '_blank')?.focus()
    notify('Opening Claude...')
    showDropdown.value = false
  } catch (err) {
    console.error('Claude error:', err)
    notify('Failed to open Claude')
  }
}

const askPerplexity = () => {
  try {
    const pageUrl = window.location.href
    const prompt = `Please help me understand this documentation page:\n${pageUrl}\n\n`
    const url = `https://www.perplexity.ai/search?q=${encodeURIComponent(prompt)}`
    window.open(url, '_blank')?.focus()
    notify('Opening Perplexity...')
    showDropdown.value = false
  } catch (err) {
    console.error('Perplexity error:', err)
    notify('Failed to open Perplexity')
  }
}

onMounted(() => {
  document.addEventListener('click', closeDropdown)
})

onUnmounted(() => {
  document.removeEventListener('click', closeDropdown)
})
</script>

<style scoped>
.page-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  position: relative;
}

.action-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 14px;
  font-weight: 500;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-border);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background: var(--vp-c-brand-soft);
  border-color: var(--vp-c-brand-1);
}

.ai-dropdown {
  position: relative;
}

.chevron {
  transition: transform 0.2s;
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  min-width: 140px;
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  z-index: 100;
  overflow: hidden;
}

.dark .dropdown-menu {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.dropdown-item {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 10px 12px;
  font-size: 14px;
  color: var(--vp-c-text-1);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
  text-align: left;
}

.dropdown-item:hover {
  background: var(--vp-c-bg-soft);
}

.notification {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 12px 20px;
  font-size: 14px;
  font-weight: 500;
  color: white;
  background: var(--vp-c-brand-1);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
  z-index: 1000;
  animation: slideIn 0.2s ease-out;
}

@keyframes slideIn {
  from {
    transform: translateY(100%);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}
</style>
