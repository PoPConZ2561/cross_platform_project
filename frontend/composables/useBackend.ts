// ตัวกลางเรียก Go (Wails). ถ้ารันใน browser ปกติ (nuxt dev) จะใช้ข้อมูลจำลองแทน
// เพื่อให้พัฒนา UI ได้โดยไม่ต้องเปิด Wails

export interface PhotoItem {
  id: number
  fileName: string
  description: string
  tags: string[]
  createdAt: string
}
export interface TagCount { name: string; count: number }
export interface Progress { done: number; total: number; skipped: number; current: string }
export interface BackupResult { copied: number; skipped: number; failed: number; errors: string[] }

const win = () => (typeof window !== 'undefined' ? (window as any) : {})
const app = () => win().go?.main?.App

// ---------- ข้อมูลจำลอง ----------
const MOCK_TAGS = ['ทะเล', 'ภูเขา', 'อาหาร', 'ครอบครัว', 'แมว', 'ถนน', 'พระอาทิตย์ตก', 'เพื่อน']
const mockPhotos: PhotoItem[] = Array.from({ length: 24 }, (_, i) => ({
  id: i + 1,
  fileName: `IMG_${String(1000 + i)}.jpg`,
  description: `ภาพถ่ายตัวอย่างที่ ${i + 1} เกี่ยวกับ${MOCK_TAGS[i % 8]}`,
  tags: [MOCK_TAGS[i % 8], MOCK_TAGS[(i * 3 + 1) % 8]],
  createdAt: new Date(2026, 0, 1 + i).toISOString()
}))
const hues = [190, 28, 140, 340, 60, 260, 12, 210]
const mockUrl = (id: number) => {
  const h = hues[id % 8]
  const svg = `<svg xmlns='http://www.w3.org/2000/svg' width='400' height='300'><rect width='400' height='300' fill='hsl(${h},35%,70%)'/><circle cx='${80 + (id * 37) % 240}' cy='120' r='60' fill='hsl(${h},40%,45%)'/></svg>`
  return 'data:image/svg+xml;utf8,' + encodeURIComponent(svg)
}
const sleep = (ms: number) => new Promise(r => setTimeout(r, ms))

export function useBackend() {
  const isWails = !!app()

  /** เปิดหน้าต่างเลือกโฟลเดอร์ -> Go: SelectFolder(title string) string */
  async function pickFolder(title: string): Promise<string> {
    if (isWails) return app().SelectFolder(title)
    await sleep(200)
    return title.includes('ต้นทาง') ? 'E:\\DCIM' : 'D:\\Backup\\Photos'
  }

  async function pickImages(): Promise<string[]> {
    if (isWails) return (await app().SelectImages()) ?? []
    await sleep(200)
    return ['D:\\Photos\\IMG_1001.jpg']
  }

  /** เริ่ม backup -> Go: StartBackup(src, dst string) BackupResult
   *  ระหว่างทำงาน Go ต้อง runtime.EventsEmit(ctx, "backup:progress", Progress) */
  async function startBackup(src: string, dst: string): Promise<BackupResult> {
    if (isWails) return app().StartBackup(src, dst)
    const total = 40
    for (let i = 1; i <= total; i++) {
      await sleep(80)
      mockEmit({ done: i, total, skipped: Math.floor(i / 10), current: `IMG_${2000 + i}.jpg` })
    }
    return { copied: 36, skipped: 4, failed: 0, errors: [] }
  }

  async function startSelectedBackup(files: string[], dst: string): Promise<BackupResult> {
    if (isWails) return app().StartBackupFiles(files, dst)
    for (let i = 0; i < files.length; i++) {
      await sleep(200)
      mockEmit({ done: i + 1, total: files.length, skipped: 0, current: files[i].split(/[\\/]/).pop() ?? files[i] })
    }
    return { copied: files.length, skipped: 0, failed: 0, errors: [] }
  }

  let mockListeners: ((p: Progress) => void)[] = []
  function mockEmit(p: Progress) { mockListeners.forEach(f => f(p)) }

  /** ฟัง event ความคืบหน้า คืนค่าฟังก์ชันสำหรับยกเลิกการฟัง */
  function onProgress(cb: (p: Progress) => void): () => void {
    if (isWails) return win().runtime.EventsOn('backup:progress', cb)
    mockListeners.push(cb)
    return () => { mockListeners = mockListeners.filter(f => f !== cb) }
  }

  /** Go: ListImages() []PhotoItem */
  async function listImages(): Promise<PhotoItem[]> {
    if (isWails) return (await app().ListImages()) ?? []
    return mockPhotos
  }
  /** Go: SearchByTag(tag string) []PhotoItem */
  async function searchByTag(tag: string): Promise<PhotoItem[]> {
    if (isWails) return (await app().SearchByTag(tag)) ?? []
    return mockPhotos.filter(p => p.tags.includes(tag))
  }
  /** Go: SearchByDescription(q string) []PhotoItem */
  async function searchByDescription(q: string): Promise<PhotoItem[]> {
    if (isWails) return (await app().SearchByDescription(q)) ?? []
    return mockPhotos.filter(p => p.description.includes(q))
  }
  /** Go: TagCloud() []TagCount */
  async function tagCloud(): Promise<TagCount[]> {
    if (isWails) return (await app().TagCloud()) ?? []
    const m = new Map<string, number>()
    mockPhotos.forEach(p => p.tags.forEach(t => m.set(t, (m.get(t) ?? 0) + 1)))
    return [...m].map(([name, count]) => ({ name, count }))
  }

  async function updateImage(id: number, description: string, tags: string[]): Promise<void> {
    if (isWails) return app().UpdateImage(id, description, tags)
    const photo = mockPhotos.find(p => p.id === id)
    if (photo) {
      photo.description = description
      photo.tags = [...new Set(tags.map(t => t.trim()).filter(Boolean))]
    }
  }

  async function deleteImage(id: number): Promise<void> {
    if (isWails) return app().DeleteImage(id)
    const index = mockPhotos.findIndex(p => p.id === id)
    if (index >= 0) mockPhotos.splice(index, 1)
  }

  async function deleteAllImages(): Promise<number> {
    if (isWails) return app().DeleteAllImages()
    const count = mockPhotos.length
    mockPhotos.splice(0, mockPhotos.length)
    return count
  }

  /** URL รูป: Wails ให้ AssetServer Handler ฝั่ง Go เสิร์ฟที่ /image/{id}?thumb=1 */
  function imageUrl(id: number, thumb = true): string {
    return isWails ? `/image/${id}${thumb ? '?thumb=1' : ''}` : mockUrl(id)
  }

  return { isWails, pickFolder, pickImages, startBackup, startSelectedBackup, onProgress, listImages, searchByTag, searchByDescription, tagCloud, updateImage, deleteImage, deleteAllImages, imageUrl }
}
