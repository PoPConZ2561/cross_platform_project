<template>
  <h1>คลังรูปและค้นหา</h1>
  <p class="sub">ค้นหารูปจาก Tag หรือคำอธิบายภาพ</p>

  <div class="tabs" role="tablist">
    <button :class="{ on: mode === 'tag' }" @click="setMode('tag')">ค้นหาจาก Tag</button>
    <button :class="{ on: mode === 'desc' }" @click="setMode('desc')">ค้นหาจากคำอธิบาย</button>
  </div>

  <section v-if="mode === 'tag'" class="panel">
    <h2>Tag ทั้งหมด</h2>
    <TagCloud :tags="tags" :selected="selectedTag" @select="pickTag" />
  </section>

  <section v-else class="panel">
    <form class="row" @submit.prevent="searchDesc">
      <input v-model="query" type="text" placeholder="พิมพ์คำที่อยู่ในคำอธิบายภาพ เช่น ทะเล" aria-label="คำค้นหา" />
      <button type="submit">ค้นหา</button>
    </form>
  </section>

  <div class="gallery-heading">
    <h2>{{ heading }}</h2>
    <button v-if="totalImages > 0" class="danger" :disabled="loading || deletingAll" @click="removeAllImages">{{
      deletingAll ? 'กำลังลบรูปทั้งหมด…' : `ลบรูปทั้งหมด (${totalImages})` }}</button>
  </div>
  <p v-if="!current && actionError" class="err">{{ actionError }}</p>
  <p v-if="loading" class="empty">กำลังโหลด…</p>
  <ImageGrid v-else :items="photos" :empty-text="emptyText" @open="current = $event" />

  <div v-if="current" class="modal" @click.self="current = null; confirmingDelete = false">
    <div class="box" role="dialog" aria-modal="true">
      <img :src="imageUrl(current.id, false)" :alt="current.description || current.fileName" />
      <div class="info">
        <h2>{{ current.fileName }}</h2>
        <p v-if="!editing">{{ current.description || 'ยังไม่มีคำอธิบาย' }}</p>
        <div v-if="!editing" class="chips">
          <button v-for="t in current.tags" :key="t" class="chip" style="border:0;cursor:pointer"
            @click="jumpToTag(t)">{{ t }}</button>
        </div>
        <form v-if="editing" class="edit-form" @submit.prevent="saveImage">
          <label>คำอธิบาย
            <textarea v-model="editDescription" rows="3" />
          </label>
          <label>Tag (คั่นด้วยจุลภาค)
            <input v-model="editTags" type="text" />
          </label>
          <p v-if="actionError" class="err">{{ actionError }}</p>
          <button type="submit" :disabled="saving">{{ saving ? 'กำลังบันทึก…' : 'บันทึก' }}</button>
          <button type="button" class="ghost" :disabled="saving" @click="editing = false">ยกเลิก</button>
        </form>
        <p v-if="!editing && actionError" class="err">{{ actionError }}</p>
        <div v-if="!editing" class="row">
          <button class="ghost" @click="beginEdit">แก้ไขข้อมูล</button>
          <button v-if="!confirmingDelete" class="ghost" @click="confirmingDelete = true">ลบรูป</button>
          <template v-else>
            <button type="button" style="background:#dc2626;color:#fff" @click="removeImage">ยืนยันลบจริง</button>
            <button type="button" class="ghost" @click="confirmingDelete = false">ยกเลิก</button>
          </template>
          <button class="ghost" @click="current = null; confirmingDelete = false">ปิด</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PhotoItem, TagCount } from '~/composables/useBackend'
const { listImages, searchByTag, searchByDescription, tagCloud, updateImage, deleteImage, deleteAllImages, imageUrl } = useBackend()

const mode = ref<'tag' | 'desc'>('tag')
const tags = ref<TagCount[]>([])
const photos = ref<PhotoItem[]>([])
const selectedTag = ref('')
const query = ref('')
const loading = ref(false)
const current = ref<PhotoItem | null>(null)
const searched = ref(false)
const editing = ref(false)
const saving = ref(false)
const confirmingDelete = ref(false)
const actionError = ref('')
const editDescription = ref('')
const editTags = ref('')
const totalImages = ref(0)
const deletingAll = ref(false)

const heading = computed(() =>
  mode.value === 'tag' && selectedTag.value ? `รูปที่มี Tag “${selectedTag.value}” (${photos.value.length})`
    : searched.value ? `ผลการค้นหา (${photos.value.length})`
      : `รูปทั้งหมด (${photos.value.length})`)
const emptyText = computed(() => searched.value ? 'ไม่พบรูปที่ตรงกับคำค้น ลองใช้คำอื่น' : 'ยังไม่มีรูป ไปที่หน้า “สำรองรูป” เพื่อเริ่มต้น')

async function load(fn: () => Promise<PhotoItem[]>) {
  loading.value = true
  try { photos.value = await fn() } finally { loading.value = false }
}
async function setMode(m: 'tag' | 'desc') {
  mode.value = m; selectedTag.value = ''; query.value = ''; searched.value = false
  await load(listImages)
}
async function pickTag(name: string) {
  selectedTag.value = name; searched.value = true
  await load(() => searchByTag(name))
}
async function jumpToTag(name: string) {
  current.value = null; editing.value = false; mode.value = 'tag'
  await pickTag(name)
}
async function searchDesc() {
  const q = query.value.trim()
  if (!q) return setMode('desc')
  searched.value = true
  await load(() => searchByDescription(q))
}

function beginEdit() {
  if (!current.value) return
  actionError.value = ''
  confirmingDelete.value = false
  editDescription.value = current.value.description
  editTags.value = current.value.tags.join(', ')
  editing.value = true
}

async function refreshResults() {
  const allImages = await listImages()
  totalImages.value = allImages.length
  tags.value = await tagCloud()
  if (selectedTag.value) {
    await load(() => searchByTag(selectedTag.value))
  } else if (mode.value === 'desc' && searched.value && query.value.trim()) {
    await load(() => searchByDescription(query.value.trim()))
  } else {
    photos.value = allImages
  }
}

async function saveImage() {
  if (!current.value) return
  saving.value = true
  actionError.value = ''
  try {
    const tags = [...new Set(editTags.value.split(',').map(tag => tag.trim()).filter(Boolean))]
    await updateImage(current.value.id, editDescription.value, tags)
    current.value.description = editDescription.value
    current.value.tags = tags
    editing.value = false
    await refreshResults()
  } catch (error: unknown) {
    actionError.value = `บันทึกข้อมูลไม่สำเร็จ: ${String(error)}`
  } finally {
    saving.value = false
  }
}

async function removeImage() {
  if (!current.value) return
  actionError.value = ''
  try {
    await deleteImage(current.value.id)
    current.value = null
    editing.value = false
    confirmingDelete.value = false
    await refreshResults()
  } catch (error: unknown) {
    actionError.value = `ลบรูปไม่สำเร็จ: ${String(error)}`
  }
}

async function removeAllImages() {
  if (totalImages.value === 0 || deletingAll.value) return
  const confirmed = window.confirm(
    `ยืนยันลบรูปทั้งหมด ${totalImages.value} รูป รวมถึงไฟล์สำรองถาวรหรือไม่? การดำเนินการนี้ไม่สามารถย้อนกลับได้`
  )
  if (!confirmed) return

  deletingAll.value = true
  actionError.value = ''
  try {
    const deleted = await deleteAllImages()
    current.value = null
    editing.value = false
    selectedTag.value = ''
    query.value = ''
    searched.value = false
    mode.value = 'tag'
    await refreshResults()
    actionError.value = `ลบรูปและไฟล์สำรองแล้ว ${deleted} รูป`
  } catch (error: unknown) {
    actionError.value = `ลบรูปทั้งหมดไม่สำเร็จ: ${String(error)}`
    try {
      await refreshResults()
    } catch (refreshError: unknown) {
      actionError.value += ` (และโหลดรายการใหม่ไม่สำเร็จ: ${String(refreshError)})`
    }
  } finally {
    deletingAll.value = false
  }
}

onMounted(async () => {
  try {
    await refreshResults()
  } catch (error: unknown) {
    actionError.value = `โหลดคลังรูปไม่สำเร็จ: ${String(error)}`
  }
})
</script>
