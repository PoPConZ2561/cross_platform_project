<template>
  <h1>สำรองรูปภาพ</h1>
  <p class="sub">เลือกโฟลเดอร์ใน flash drive หรือโทรศัพท์ แล้วเลือกที่เก็บรูปบนเครื่อง</p>

  <section class="panel">
    <h2>รูปภาพต้นทาง หรือ โฟลเดอร์ต้นทาง</h2>
    <div class="row">
      <button class="ghost" :disabled="running" @click="chooseImages">เลือกรูปภาพ (เลือกได้หลายรูป)</button>
      <button class="ghost" :disabled="running" @click="chooseSrc">เลือกโฟลเดอร์</button>
    </div>
    <p v-if="src" class="path source-selection">{{ src }}</p>
    <div v-else-if="selectedFiles.length" class="selected-files">
      <p>เลือกแล้ว {{ selectedFiles.length }} รูป</p>
      <ul>
        <li v-for="file in selectedFiles" :key="file">
          <span class="path">{{ file }}</span>
          <button class="ghost" :disabled="running" :aria-label="`นำ ${file} ออกจากรายการ`" @click="removeSelectedFile(file)">นำออก</button>
        </li>
      </ul>
      <button class="ghost" :disabled="running" @click="clearSelectedFiles">ล้างรายการรูป</button>
    </div>
    <p v-else class="path source-selection">ยังไม่ได้เลือกรูปภาพหรือโฟลเดอร์</p>
  </section>

  <section class="panel">
    <h2>โฟลเดอร์ปลายทาง</h2>
    <div class="row">
      <button class="ghost" :disabled="running || checkingDestination" @click="chooseDst">เลือกโฟลเดอร์</button>
      <p class="path">{{ dst || 'ยังไม่ได้เลือก' }}</p>
    </div>
    <p v-if="checkingDestination" class="path">กำลังตรวจสอบประวัติฐานข้อมูลกับไฟล์ในโฟลเดอร์ปลายทาง…</p>
    <p v-else-if="destinationCheckError" class="err">ตรวจสอบไฟล์ไม่สำเร็จ: {{ destinationCheckError }}</p>
    <template v-else-if="destinationChecked && missingFiles.length">
      <p class="err">พบไฟล์สูญหาย {{ missingFiles.length }} ไฟล์ — มีประวัติในฐานข้อมูลแต่ไม่พบไฟล์จริง</p>
      <ul class="err">
        <li v-for="file in missingFiles" :key="file.path">{{ file.fileName }} — {{ file.path }}</li>
      </ul>
    </template>
    <p v-else-if="destinationChecked" class="path">ตรวจสอบแล้ว: ไม่พบไฟล์สูญหาย</p>
  </section>

  <section class="panel">
    <div class="row">
      <button :disabled="(!src && !selectedFiles.length) || !dst || running" @click="run">
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
    <p v-if="result" class="path" style="margin-top:.75rem">
      ใช้เวลารวม {{ (result.durationMillis / 1000).toFixed(3) }} วินาที ({{ result.durationMillis }} ms)
    </p>
    <ul v-if="result?.errors.length" class="err" style="margin-top:1rem">
      <li v-for="item in result.errors" :key="item">{{ item }}</li>
    </ul>
    <p v-if="error" class="err" style="margin-top:1rem">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import type { MissingFile, Progress, BackupResult } from '~/composables/useBackend'
const { pickFolder, pickImages, startBackup, startSelectedBackup, checkDestination, onProgress } = useBackend()

const src = ref(''), dst = ref('')
const selectedFiles = ref<string[]>([])
const running = ref(false)
const error = ref('')
const result = ref<BackupResult | null>(null)
const progress = reactive<Progress>({ done: 0, total: 0, skipped: 0, current: '' })
const missingFiles = ref<MissingFile[]>([])
const checkingDestination = ref(false)
const destinationChecked = ref(false)
const destinationCheckError = ref('')
const percent = computed(() => (progress.total ? Math.round((progress.done / progress.total) * 100) : 0))

async function chooseSrc() {
  const p = await pickFolder('เลือกโฟลเดอร์ต้นทาง')
  if (p) {
    src.value = p
    selectedFiles.value = []
  }
}
async function chooseDst() {
  try {
    const folder = await pickFolder('เลือกโฟลเดอร์ปลายทาง')
    if (!folder) return
    dst.value = folder
    checkingDestination.value = true
    destinationChecked.value = false
    destinationCheckError.value = ''
    missingFiles.value = await checkDestination(folder)
    destinationChecked.value = true
  } catch (e: unknown) {
    destinationCheckError.value = String(e)
    destinationChecked.value = true
  } finally {
    checkingDestination.value = false
  }
}
async function chooseImages() {
  try {
    const files = await pickImages()
    if (files.length) {
      error.value = ''
      src.value = ''
      selectedFiles.value = [...new Set([...selectedFiles.value, ...files])]
    }
  } catch (e: unknown) {
    error.value = 'เลือกรูปภาพไม่สำเร็จ: ' + String(e)
  }
}
function removeSelectedFile(file: string) {
  selectedFiles.value = selectedFiles.value.filter(selected => selected !== file)
}
function clearSelectedFiles() {
  selectedFiles.value = []
}

let off: (() => void) | undefined
onMounted(() => { off = onProgress(p => Object.assign(progress, p)) })
onUnmounted(() => off?.())

async function run() {
  running.value = true; error.value = ''; result.value = null
  Object.assign(progress, { done: 0, total: 0, skipped: 0, current: '' })
  try {
    result.value = src.value
      ? await startBackup(src.value, dst.value)
      : await startSelectedBackup(selectedFiles.value, dst.value)
  } catch (e: unknown) {
    error.value = 'สำรองรูปไม่สำเร็จ: ' + String(e) + ' — ตรวจสอบว่าอุปกรณ์ยังเชื่อมต่ออยู่ แล้วลองใหม่'
  } finally {
    running.value = false
  }
}
</script>
