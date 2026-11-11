<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

export interface SelectOption {
  label: string
  value: string | number
  sublabel?: string
  icon?: string
}

const props = withDefaults(defineProps<{
  modelValue: string | number
  options: (string | number | SelectOption)[]
  placeholder?: string
  clearable?: boolean
  size?: 'sm' | 'md'
  icon?: string
  minWidth?: string
  disabled?: boolean
}>(), {
  placeholder: '请选择...',
  clearable: false,
  size: 'md',
  minWidth: '150px',
  disabled: false
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number): void
  (e: 'change', value: string | number): void
}>()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

// Normalize options to uniform { label, value, sublabel?, icon? }
const normalizedOptions = computed<SelectOption[]>(() => {
  return props.options.map(opt => {
    if (typeof opt === 'object' && opt !== null) {
      return opt as SelectOption
    }
    return {
      label: String(opt),
      value: opt
    }
  })
})

// Current selected option object
const selectedOption = computed(() => {
  return normalizedOptions.value.find(opt => opt.value === props.modelValue)
})

// Display text in trigger
const displayText = computed(() => {
  if (selectedOption.value) {
    return selectedOption.value.label
  }
  if (props.modelValue !== '' && props.modelValue !== undefined && props.modelValue !== null) {
    return String(props.modelValue)
  }
  return props.placeholder
})

const hasValue = computed(() => {
  return props.modelValue !== '' && props.modelValue !== undefined && props.modelValue !== null
})

function toggleDropdown() {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}

function selectOption(opt: SelectOption) {
  if (props.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
  isOpen.value = false
}

function handleClear(e: Event) {
  e.stopPropagation()
  emit('update:modelValue', '')
  emit('change', '')
  isOpen.value = false
}

function handleClickOutside(e: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

onMounted(() => {
  window.addEventListener('click', handleClickOutside)
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('click', handleClickOutside)
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div 
    ref="containerRef" 
    :class="['firefly-select', `size-${size}`, { 'is-open': isOpen, 'is-disabled': disabled, 'has-value': hasValue }]"
    :style="{ minWidth }"
  >
    <!-- Select Trigger -->
    <button 
      type="button" 
      class="select-trigger" 
      @click="toggleDropdown" 
      :disabled="disabled"
      :aria-expanded="isOpen"
    >
      <span v-if="icon || selectedOption?.icon" class="material-symbols-rounded select-prefix-icon">
        {{ selectedOption?.icon || icon }}
      </span>

      <span :class="['select-label', { 'is-placeholder': !hasValue }]">
        {{ displayText }}
      </span>

      <!-- Action Icons (Clear / Arrow) -->
      <span class="select-actions">
        <span 
          v-if="clearable && hasValue && !disabled" 
          role="button"
          tabindex="0"
          class="clear-btn" 
          @click="handleClear" 
          @keydown.enter.prevent="handleClear"
          @keydown.space.prevent="handleClear"
          title="清空选择"
        >
          <span class="material-symbols-rounded">close</span>
        </span>
        <span class="material-symbols-rounded arrow-icon">expand_more</span>
      </span>
    </button>

    <!-- Floating Dropdown Menu -->
    <Transition name="firefly-popover">
      <div v-if="isOpen" class="select-dropdown-menu">
        <div class="menu-specular-top"></div>
        <div class="menu-scroll-container">
          <button
            v-for="opt in normalizedOptions"
            :key="String(opt.value)"
            type="button"
            :class="['menu-option-item', { 'is-selected': opt.value === modelValue }]"
            @click="selectOption(opt)"
          >
            <span v-if="opt.icon" class="material-symbols-rounded option-icon">{{ opt.icon }}</span>
            <div class="option-content">
              <span class="option-label">{{ opt.label }}</span>
              <span v-if="opt.sublabel" class="option-sub">{{ opt.sublabel }}</span>
            </div>
            <span v-if="opt.value === modelValue" class="material-symbols-rounded check-icon">check</span>
          </button>
          <div v-if="normalizedOptions.length === 0" class="empty-options">
            <span>暂无选项</span>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.firefly-select {
  position: relative;
  display: inline-block;
  user-select: none;
  font-family: 'Outfit', sans-serif;
}

.select-trigger {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  color: #fff;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
  outline: none;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  text-align: left;
}

.size-sm .select-trigger {
  padding: 5px 10px;
  font-size: 12px;
  border-radius: 8px;
}

.select-trigger:hover:not(:disabled) {
  background: rgba(0, 229, 255, 0.06);
  border-color: rgba(0, 229, 255, 0.3);
  color: #fff;
}

.firefly-select.is-open .select-trigger {
  background: rgba(0, 229, 255, 0.08);
  border-color: hsl(var(--md-sys-color-primary));
  box-shadow: 0 0 16px -2px rgba(0, 229, 255, 0.3), inset 0 1px 1px rgba(255, 255, 255, 0.1);
}

.select-prefix-icon {
  font-size: 16px;
  color: hsl(var(--md-sys-color-primary));
  flex-shrink: 0;
}

.select-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 500;
  color: #fff;
}

.select-label.is-placeholder {
  color: rgba(255, 255, 255, 0.45);
  font-weight: 400;
}

.select-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  margin-left: auto;
}

.clear-btn {
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border-radius: 50%;
  transition: all 0.15s ease;
}

.clear-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.15);
}

.clear-btn .material-symbols-rounded {
  font-size: 14px;
}

.arrow-icon {
  font-size: 18px;
  color: rgba(255, 255, 255, 0.45);
  transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1), color 0.2s ease;
}

.is-open .arrow-icon {
  transform: rotate(180deg);
  color: hsl(var(--md-sys-color-primary));
}

/* Floating Dropdown Menu Panel */
.select-dropdown-menu {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  right: 0;
  min-width: 100%;
  width: max-content;
  max-width: 320px;
  background: rgba(10, 16, 24, 0.96);
  backdrop-filter: blur(28px);
  -webkit-backdrop-filter: blur(28px);
  border: 1px solid rgba(0, 229, 255, 0.18);
  border-radius: 12px;
  box-shadow: 0 16px 36px -4px rgba(0, 0, 0, 0.85),
              0 0 20px -2px rgba(0, 229, 255, 0.18),
              inset 0 1px 1px 0 rgba(255, 255, 255, 0.15);
  z-index: 120;
  overflow: hidden;
  padding: 4px;
}

.menu-specular-top {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(0, 229, 255, 0.5), transparent);
}

.menu-scroll-container {
  max-height: 240px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.menu-option-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.8);
  cursor: pointer;
  text-align: left;
  font-size: 13px;
  transition: all 0.18s ease;
}

.menu-option-item:hover {
  background: rgba(0, 229, 255, 0.1);
  color: #fff;
}

.menu-option-item.is-selected {
  background: rgba(0, 229, 255, 0.15);
  color: hsl(var(--md-sys-color-primary));
  font-weight: 600;
}

.option-icon {
  font-size: 16px;
  color: rgba(255, 255, 255, 0.5);
  flex-shrink: 0;
}

.menu-option-item.is-selected .option-icon {
  color: hsl(var(--md-sys-color-primary));
}

.option-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.option-label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.option-sub {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.4);
}

.check-icon {
  font-size: 16px;
  color: hsl(var(--md-sys-color-primary));
  flex-shrink: 0;
  margin-left: auto;
}

.empty-options {
  padding: 16px 12px;
  text-align: center;
  color: rgba(255, 255, 255, 0.4);
  font-size: 12px;
}

/* Transitions */
.firefly-popover-enter-active,
.firefly-popover-leave-active {
  transition: opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1),
              transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.firefly-popover-enter-from {
  opacity: 0;
  transform: translateY(-8px) scale(0.96);
}

.firefly-popover-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
