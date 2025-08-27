<template>
  <div class="text-white">
    <header class="flex justify-between items-center bg-[rgb(19,26,37)]" style="--wails-draggable: drag">
      <div class="w-fit flex gap-2 items-center ml-2">
        <img class="w-[18px] h-[18px] mt-2 mb-2" src="/src/assets/images/appicon.png" />
        <span class="text-[18px] font-semibold pointer-events-none">GreenLuma</span>
      </div>
      <div class="flex items-center">
        <button @click="WindowMinimise" class="p-2">
          <Minus :size="15"/>
        </button>
        <button @click="WindowToggleMaximise" class="p-2">
          <Maximize2 :size="15"/>
        </button>
        <button @click="Quit" class="p-2">
          <X :size="15"/>
        </button>
      </div>
    </header>
    <div class="p-4">
      <!-- <h1 class="text-xl font-bold text-center w-full pb-2.5">GreenLuma</h1> -->

      <div v-if="!steamDir" class="mt-4">
        <p>Пожалуйста, выберите папку Steam...</p>
        <button @click="selectSteamDir" class="px-3 py-1 rounded mt-2 bg-gray-600 active:bg-gray-800 hover:bg-gray-700">
          Указать папку Steam
        </button>
      </div>

      <div v-else>
        <!-- <p class="mb-2">Steam папка: {{ steamDir }}</p>
        <button @click="selectSteamDir" class="px-3 py-1 rounded mb-2">
          Изменить папку Steam
        </button> -->

        <div class="flex gap-2 mb-2">
          <input v-model="searchQuery" @keyup.enter="searchGames" type="text" placeholder="Поиск" class="flex-1 p-2 rounded bg-[rgb(35,46,66)] border border-gray-700 focus:border-white outline-none transition-colors duration-300"/>
          <button @click="searchGames"
          class="px-2 py-0.5 rounded bg-gray-600 active:bg-gray-800 hover:bg-gray-700 flex items-center justify-center"
          >
            <template v-if="isSearching">
              <svg class="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
              </svg>
            </template>
            <template v-else>
              <Search :size="20" />
            </template>
          </button>
          <button v-if="!searchQuery.trim()" @click="loadGames" class="px-2 py-0.5 rounded bg-gray-600 active:bg-gray-800 hover:bg-gray-700"><RefreshCcw :size="20" /></button>
        </div>

        <div v-if="games.length === 0" class="mt-4">
          <p>Нет игр.</p>
        </div>

        <transition-group name="games" tag="div" class="space-y-2">
          <div v-for="game in games" :key="game.appid" class="flex items-stretch border border-gray-700 bg-[rgb(35,46,66)] shadow-xl mt-2 rounded overflow-hidden">
            <!-- левая часть -->
            <div class="flex items-center gap-3 flex-1 p-2">
              <img :src="game.image" class="w-20 h-10 object-cover rounded" />
              <div>
                <p class="font-semibold">{{ game.name }}</p>
                <p class="text-gray-500 text-sm">AppID: {{ game.appid }}</p>
              </div>
            </div>

            <!-- правая часть -->
            <div class="flex">
              <button @click="game.installed ? removeGame(game.appid) : addGame(game)" :class="[
                'transition-all duration-300 ease-in-out flex items-center justify-center',
                game.installed
                ? 'bg-red-400 active:bg-red-600, hover:bg-red-500 px-2' : 'bg-[rgb(90,150,0)] active:bg-[rgb(60,100,0)] hover:bg-[rgb(75,125,0)] px-4'
              ]" style="display: inline-flex;">
                <component :is="game.installed ? CopyMinus : CopyPlus" :size="20" />
              </button>
            </div>
          </div>
        </transition-group>


        <!-- статус бар -->
        <div class="fixed bottom-0 left-0 w-full h-8 flex items-center justify-between pl-1.5 pr-1.5" :class="isDllInstalled ? 'bg-[rgb(90,150,0)]' : 'bg-red-400'">
          <span class="font-semibold">
            {{ tempStatusText }}
          </span>
          <button v-if="!isDllInstalled" @click="installDll" class="px-3 py-1 rounded bg-gray-600 active:bg-gray-800 hover:bg-gray-700" title="Установить GreenLuma">
            <PackagePlus :size="15" />
          </button>
          <div v-else>
            <button @click="clearCache" class="px-3 py-1 rounded mr-1.5 bg-gray-600 active:bg-gray-800 hover:bg-gray-700" title="Очистить кэш Steam">
              <Bubbles :size="15" />
            </button>
            <button @click="removeDll" class="px-3 py-1 rounded bg-gray-600 active:bg-gray-800 hover:bg-gray-700" title="Удалить GreenLuma">
              <PackageMinus :size="15" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch, onMounted } from "vue";
import {
  Quit,
  WindowMinimise,
  WindowToggleMaximise
} from "../wailsjs/runtime/runtime.js"
import {
  GetInstalledGames,
  RemoveGame,
  AddGame,
  SearchGames,
  SelectSteamDir,
  GetSteamDir,
  IsDllInstalled,
  InstallDll,
  RemoveDll,
  DeleteSteamCache
} from "../wailsjs/go/main/App.js";
import {
  Search,
  RefreshCcw,
  X,
  Minus,
  Maximize2,
  Bubbles,
  PackageMinus,
  PackagePlus,
  CopyPlus, CopyMinus
} from "lucide-vue-next"

const steamDir = ref<string | null>(null);
const games = ref<any[]>([]);
const searchQuery = ref("");

const isDllInstalled = ref(false);
const tempStatusText = ref("");

const isSearching = ref(false);

onMounted(async () => {
  console.log('Vue mounted');
  try {
    const dir = await GetSteamDir();
    if (dir) {
      steamDir.value = dir;
      await loadGames();
    }
  } catch (e) {
    console.warn("SteamDir пустой или ошибка:", e);
    
  }

  try {
    isDllInstalled.value = await IsDllInstalled()
  } catch (e) {
    console.error("Ошибка проверки наличия dll:", e)
  }
})

async function selectSteamDir() {
  try {
    const dir = await SelectSteamDir();
    steamDir.value = dir;
    await checkDll()
    await loadGames();
  } catch (err: any) {
    alert(err.message || "Ошибка при выборе папки Steam\nERROR: " + err);
  }
}

async function loadGames() {
  try {
    const installed = await GetInstalledGames();
    games.value = sortGames(installed.map(g => ({ ...g, installed: true })));
  } catch (e) {
    console.error("Ошибка загрузки игр:", e);
  }
}

watch(searchQuery, async (newVal) =>  {
  if (!newVal.trim()) {
    await loadGames();
  }
})

async function searchGames() {
  if (!searchQuery.value.trim()) return;

  isSearching.value = true;
  try {
    const results = await SearchGames(searchQuery.value.trim());
    // проверяем, установлены ли уже эти игры
    const installed = await GetInstalledGames();
    const installedIds = new Set(installed.map(g => Number(g.appid)));

    games.value = sortGames(results.map((g: any) => ({
      ...g,
      installed: installedIds.has(Number(g.appid)),
    })));
  } catch (e) {
    console.error("Ошибка поиска игр:", e);
  } finally {
    isSearching.value = false;
  }
}

async function addGame(game: any) {
  try {
    console.log("Добавляем игру с appid:", game.appid)
    await AddGame(Number(game.appid));
    game.installed = true;
  } catch (e) {
    console.error("Ошибка добавления игры:", e);
  }
}

async function removeGame(appid: number) {
  try {
    await RemoveGame(appid);
    const game = games.value.find(g => g.appid === appid)
    if (game) game.installed = false;
  } catch (e) {
    console.error("Ошибка удаления игры:", e);
  }
}

async function checkDll() {
  try {
    isDllInstalled.value = await IsDllInstalled()
  } catch {
    isDllInstalled.value = false
  }
}

async function installDll() {
  try {
    await InstallDll()
    await checkDll()
  } catch (e) {
    console.error("Ошибка установки dll:", e)
    alert("Не удалось установить user32.dll\nERROR: " + e)
  }
}

async function removeDll() {
  try {
    await RemoveDll()
    await checkDll()
  } catch (e) {
    console.error("Ошибка удаления dll:", e)
    alert("Не удалось удалить user32.dll\nERROR: " + e)
  }
}

async function clearCache() {
  try {
    const msg = await DeleteSteamCache();
    tempStatusText.value = msg;

    setTimeout(() => {
      tempStatusText.value = "";
    }, 3000);
  } catch (e: any) {
    tempStatusText.value = "Ошибка очистки!";
    console.error(e);
    setTimeout(() => {
      tempStatusText.value = "";
    }, 3000)
  }
}

function sortGames(arr: any[]) {
  return arr.sort((a, b) => a.name.localeCompare(b.name));
}
</script>
