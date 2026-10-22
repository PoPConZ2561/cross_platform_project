<template>
  <h1>สำรองรูปภาพ</h1>
  <p class="sub">เลือกโฟลเดอร์ใน flash drive หรือโทรศัพท์ แล้วเลือกที่เก็บรูปบนเครื่อง</p>

  <section class="panel">
    <h2>โฟลเดอร์ต้นทาง</h2>
    <div class="row">
      <button class="ghost" :disabled="running" @click="chooseSrc">เลือกโฟลเดอร์</button>
      <p class="path">{{ src || 'ยังไม่ได้เลือก' }}</p>
    </div>
  </section>

  <section class="panel">
    <h2>โฟลเดอร์ปลายทาง</h2>
    <div class="row">
      <button class="ghost" :disabled="running" @click="chooseDst">เลือกโฟลเดอร์</button>
      <p class="path">{{ dst || 'ยังไม่ได้เลือก' }}</p>
    </div>
  </section>

  <section class="panel">
    <div class="row">
      <button :disabled="!src || !dst || running" @click="run">
        {{ running ? 'กำลังสำรอง…' : 'เริ่มสำรองรูป' }}
      </button>
      <p v-if="running" class="path">{{ progress.current }}</p>
    </div>

    <div v-if="running || result" style="margin-top:1rem">
      <div class="bar" role="progressbar" :aria-valuenow="percent" aria-valuemin="0" aria-valuemax="100">
        <i :style="{ width: percent + '%' }" />
      </div>
      <p class="path" style="margin-top:.4rem">{{ progress.done }} / {{ progress.total }} ไฟล์ ({{ percent }}%)</p>
    </div>

    <div v-if="result" class="stats">
      <div><span>{{ result.copied }}</span><small>คัดลอกแล้ว</small></div>
      <div><span>{{ result.skipped }}</span><small>ข้าม (ซ้ำ)</small></div>
      <div><span :class="{ err: result.failed }">{{ result.failed }}</span><small>ผิดพลาด</small></div>
    </div>
    <ul v-if="result?.errors.length" class="err" style="margin-top:1rem">
      <li v-for="item in result.errors" :key="item">{{ item }}</li>
    </ul>
    <p v-if="error" class="err" style="margin-top:1rem">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import type { Progress, BackupResult } from '~/composables/useBackend'
const { pickFolder, startBackup, onProgress } = useBackend()

const src = ref(''), dst = ref('')
const running = ref(false)
const error = ref('')
const result = ref<BackupResult | null>(null)
const progress = reactive<Progress>({ done: 0, total: 0, skipped: 0, current: '' })
const percent = computed(() => (progress.total ? Math.round((progress.done / progress.total) * 100) : 0))

async function chooseSrc() { const p = await pickFolder('เลือกโฟลเดอร์ต้นทาง'); if (p) src.value = p }
async function chooseDst() { const p = await pickFolder('เลือกโฟลเดอร์ปลายทาง'); if (p) dst.value = p }

let off: (() => void) | undefined
onMounted(() => { off = onProgress(p => Object.assign(progress, p)) })
onUnmounted(() => off?.())

async function run() {
  running.value = true; error.value = ''; result.value = null
  Object.assign(progress, { done: 0, total: 0, skipped: 0, current: '' })
  try {
    result.value = await startBackup(src.value, dst.value)
  } catch (e: any) {
    error.value = 'สำรองรูปไม่สำเร็จ: ' + (e?.message ?? e) + ' — ตรวจสอบว่าอุปกรณ์ยังเชื่อมต่ออยู่ แล้วลองใหม่'
  } finally {
    running.value = false
  }
}
</script>
