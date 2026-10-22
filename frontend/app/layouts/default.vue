<template>
  <div class="min-h-screen bg-base-200">
    <div class="navbar bg-base-100 shadow-sm px-4">
      <div class="flex-1">
        <NuxtLink to="/" class="btn btn-ghost text-xl font-bold">
          📸 Image Backup & Search
        </NuxtLink>
      </div>
      <div class="flex-none">
        <ul class="menu menu-horizontal px-1 gap-1">
          <li><NuxtLink to="/">สำรองข้อมูล (Backup)</NuxtLink></li>
          <li><NuxtLink to="/gallery">ค้นหา & Tag Cloud</NuxtLink></li>
          <li><NuxtLink to="/integrity">ตรวจสอบไฟล์ (Check)</NuxtLink></li>
        </ul>
      </div>
    </div>
    <main class="p-6 max-w-6xl mx-auto">
      <slot />
    </main>
  </div>
</template>