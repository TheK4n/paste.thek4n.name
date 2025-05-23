import type { Component } from 'solid-js';
import { createSignal, createResource, Match, Switch } from 'solid-js';

async function shortenUrl(url: string, disposable: number, ttl: string): Promise<string> {
  if (disposable > 255) {
    throw new Error("Error: disposable counter cant be more then 255");
  }

  const apiUrl = import.meta.env.VITE_API_URL
  const response = await fetch(`${apiUrl}/?url=true&ttl=${ttl}&disposable=${disposable}`, {
    method: "POST",
    mode: "cors",
    headers: {
      'Content-Type': 'text/plain',
      Accept: 'text/plain',
      'Access-Control-Request-Method': "POST",
      'Access-Control-Request-Headers': "content-type,accept",
    },
    body: url,
  });

  if (!response.ok) {
    throw new Error(`Error! status: ${response.status}`);
  }

  return await response.text();
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    alert("Copied");
  });
}

const App: Component = () => {
  const [disposableCounter, setDisposableCounter] = createSignal<number>(0);
  const [expirationTime, setExpirationTime] = createSignal<string>("");
  const [url, setURL] = createSignal<string>("");
  const [openAccordion, setOpenAccordion] = createSignal<string | null>(null);

  const toggleAccordion = (id: string) => {
    setOpenAccordion(openAccordion() === id ? null : id);
  };

  const [shortenedURL, { mutate: _, refetch }] = createResource<string, string>(
    async () => {
      if (!url()) return "";

      const fullUrl = /^https?:\/\//i.test(url()) ? url() : `https://${url()}`;
      return await shortenUrl(fullUrl, disposableCounter(), expirationTime());
    }
  );

  const handleShorten = () => {
    refetch();
  };

  return (
    <div class="min-h-screen flex flex-col">
      <main class="flex-grow container mx-auto px-4 py-8">
        <header class="max-w-2xl mx-auto">
          <div class="mb-6">
            <label for="url" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">URL</label>
            <input
              type="url"
              id="url"
              placeholder="https://example.com/"
              value={url()}
              onChange={(e) => setURL(e.currentTarget.value)}
              onKeyPress={(e) => {
                if (e.key === "Enter") {
                  setURL(e.currentTarget.value);
                  handleShorten();
                }
              }}
              required
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
            />
          </div>
          <div class="grid gap-6 mb-6 md:grid-cols-2">
            <div>
              <label for="ttl" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">URL Expiration Time</label>
              <input
                type="text"
                id="ttl"
                placeholder="24h"
                value={expirationTime()}
                onChange={(e) => setExpirationTime(e.currentTarget.value)}
                onKeyPress={(e) => {
                  if (e.key === "Enter") {
                    setExpirationTime(e.currentTarget.value);
                    handleShorten();
                  }
                }}
                required
                class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
              />
            </div>
            <div>
              <label for="visitors" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">Disposable counter</label>
              <input
                type="number"
                id="visitors"
                value={disposableCounter()}
                onInput={(e) => setDisposableCounter(Number(e.currentTarget.value))}
                onKeyPress={(e) => {
                  if (e.key === "Enter") {
                    setDisposableCounter(Number(e.currentTarget.value));
                    handleShorten();
                  }
                }}
                min="0"
                max="255"
                placeholder="0"
                required
                class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
              />
            </div>
          </div>
          <button
            onclick={handleShorten}
            class="text-white bg-blue-700 hover:bg-blue-800 focus:ring-4 focus:outline-none focus:ring-blue-300 font-medium rounded-lg text-sm w-full sm:w-auto px-5 py-2.5 text-center dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
          >
            Shorten
          </button>

          <Switch>
            <Match when={shortenedURL.loading}>
              <div class="mt-10 text-white">Loading...</div>
            </Match>
            <Match when={shortenedURL.error}>
              <div class="mt-10 text-red-500">Error: {shortenedURL.error.message}</div>
            </Match>
            <Match when={shortenedURL()}>
              <div class="mt-10">
                <div class="flex text-white items-center border border-gray-700 rounded-lg overflow-hidden">
                  <input
                    type="text"
                    value={shortenedURL()}
                    readonly
                    class="flex-grow px-4 py-2 outline-none bg-gray-800"
                  />
                  <button
                    class="bg-blue-500 text-white px-4 py-2 hover:bg-blue-600 transition-colors"
                    onclick={() => copyToClipboard(shortenedURL()!)}
                  >
                    Copy
                  </button>
                </div>
              </div>
            </Match>
          </Switch>
        </header>
      </main>

      <footer class="dark:bg-gray-800 mt-auto">
        <div class="container mx-auto px-4 py-8 max-w-4xl">
          <h2 class="text-xl font-bold mb-6 text-gray-900 dark:text-white">Legal Information</h2>

          <div class="mb-8">
            <button
              onclick={() => toggleAccordion('disclaimer')}
              class="flex justify-between items-center w-full px-4 py-3 text-left bg-gray-200 dark:bg-gray-700 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
            >
              <span class="font-medium text-gray-900 dark:text-white">Disclaimer (Отказ от ответственности)</span>
              <svg
                class={`w-5 h-5 transform transition-transform ${openAccordion() === 'disclaimer' ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
              </svg>
            </button>
            <div class={`px-4 pt-4 pb-2 ${openAccordion() === 'disclaimer' ? 'block' : 'hidden'}`}>
              <p class="mb-3 text-gray-700 dark:text-gray-300">1. Сервис предоставляется "как есть" (as is) без каких-либо гарантий, явных или подразумеваемых.</p>
              <p class="mb-3 text-gray-700 dark:text-gray-300">2. Администрация сервиса не несет ответственности за:</p>
              <ul class="list-disc pl-5 mb-3 text-gray-700 dark:text-gray-300 space-y-1">
                <li>Любые прямые или косвенные убытки, возникшие в результате использования или невозможности использования сервиса</li>
                <li>Действия пользователей, совершенные с использованием данного сервиса</li>
                <li>Точность, полноту или актуальность предоставляемой информации</li>
                <li>Доступность сервиса в любое время и без перебоев</li>
              </ul>
              <p class="text-gray-700 dark:text-gray-300">3. Пользователи осуществляют все действия на свой страх и риск.</p>
            </div>
          </div>

          <div class="mb-8">
            <button
              onclick={() => toggleAccordion('terms')}
              class="flex justify-between items-center w-full px-4 py-3 text-left bg-gray-200 dark:bg-gray-700 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
            >
              <span class="font-medium text-gray-900 dark:text-white">Terms of Service (Условия использования)</span>
              <svg
                class={`w-5 h-5 transform transition-transform ${openAccordion() === 'terms' ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
              </svg>
            </button>
            <div class={`px-4 pt-4 pb-2 ${openAccordion() === 'terms' ? 'block' : 'hidden'}`}>
              <p class="mb-3 text-gray-700 dark:text-gray-300">1. Используя сервис, вы соглашаетесь с настоящими условиями.</p>
              <p class="mb-3 text-gray-700 dark:text-gray-300">2. Запрещается:</p>
              <ul class="list-disc pl-5 mb-3 text-gray-700 dark:text-gray-300 space-y-1">
                <li>Использовать сервис для незаконной деятельности</li>
                <li>Нарушать права других пользователей или третьих лиц</li>
                <li>Распространять вредоносное ПО, спам или вирусы</li>
                <li>Пытаться получить несанкционированный доступ к данным сервиса</li>
                <li>Создавать помехи в работе сервиса</li>
              </ul>
              <p class="mb-3 text-gray-700 dark:text-gray-300">3. Администрация оставляет за собой право:</p>
              <ul class="list-disc pl-5 text-gray-700 dark:text-gray-300 space-y-1">
                <li>Блокировать доступ пользователям, нарушающим правила</li>
                <li>Изменять функционал сервиса без предварительного уведомления</li>
                <li>Прекратить работу сервиса в любой момент</li>
              </ul>
            </div>
          </div>

          <div class="text-center text-sm text-gray-500 dark:text-gray-400 mt-8">
            <p>© {new Date().getFullYear()} URL Shortener Service. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </div>
  );
};

export default App;
