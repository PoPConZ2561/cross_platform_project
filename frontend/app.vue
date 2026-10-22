<template>
  <div class="shell">
    <aside class="side">
      <div class="brand">Photo Backup</div>
      <nav>
        <NuxtLink to="/">สำรองรูป</NuxtLink>
        <NuxtLink to="/gallery">คลังรูปและค้นหา</NuxtLink>
      </nav>
      <p class="mode" v-if="!isWails">โหมดทดสอบ (ข้อมูลจำลอง)</p>
    </aside>
    <main class="main"><NuxtPage /></main>
  </div>
</template>

<script setup lang="ts">
const { isWails } = useBackend()
</script>
