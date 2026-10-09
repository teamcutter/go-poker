import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { test } from 'node:test'
import ts from 'typescript'

const source = await readFile(new URL('../src/telegram.ts', import.meta.url), 'utf8')
let serial = 0

async function telegram(env, webApp) {
  globalThis.__scenarioEnv = env
  globalThis.window = webApp ? { Telegram: { WebApp: webApp } } : {}
  const transformed = ts.transpileModule(source.replaceAll('import.meta.env', 'globalThis.__scenarioEnv'), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 },
  }).outputText
  return import(`data:text/javascript,${encodeURIComponent(transformed)}#${++serial}`)
}

test('TC-20014 invite deep link carries the table code', async () => {
  const { inviteLink } = await telegram({ DEV: false, VITE_BOT_USERNAME: '@PokerBot', VITE_MINIAPP_NAME: 'play' })
  assert.equal(inviteLink('AB23'), 'https://t.me/PokerBot/play?startapp=AB23')
})

test('TC-20014 missing bot username falls back to the table code', async () => {
  const { inviteLink, shareInvite } = await telegram({ DEV: false }, { openTelegramLink() { assert.fail('unexpected share') } })
  assert.equal(inviteLink('AB23'), '')
  assert.equal(shareInvite('AB23', 'Join my table'), false)
})

test('TC-20014 share opens Telegram picker when a link is available', async () => {
  let opened = ''
  const { shareInvite } = await telegram({ DEV: false, VITE_BOT_USERNAME: 'PokerBot' }, {
    openTelegramLink(url) { opened = url },
  })
  assert.equal(shareInvite('AB23', 'Join my table'), true)
  const link = new URL(opened)
  assert.equal(link.origin + link.pathname, 'https://t.me/share/url')
  assert.equal(link.searchParams.get('url'), 'https://t.me/PokerBot?startapp=AB23')
  assert.equal(link.searchParams.get('text'), 'Join my table')
})
