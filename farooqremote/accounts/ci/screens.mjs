// Farooq Remote (FA-020): pictures of the admin page in English, Urdu and Arabic - the build's own check of the
// right-to-left layout, and the pictures for the help pages. Uses the demo data from ci/demo.sql.
// env: BASE (http://127.0.0.1:21114/_admin/), FR_USER, FR_PASS, OUT (folder)
import { chromium } from 'playwright'
import { mkdirSync, writeFileSync } from 'node:fs'

const base = process.env.BASE || 'http://127.0.0.1:21114/_admin/'
const out = process.env.OUT || 'screens'
mkdirSync(out, { recursive: true })
const pages = [
  ['01-my-info', '#/'],
  ['02-my-devices', '#/my/peer'],
  ['03-address-book', '#/my/address_book'],
  ['04-tags', '#/my/tag'],
  ['05-login-log', '#/my/loginLog'],
  ['06-users', '#/user/index'],
  ['07-all-devices', '#/user/peer'],
]
const settle = async (page) => {
  await page.waitForLoadState('networkidle').catch(() => {})
  await page.waitForTimeout(900)
}
const browser = await chromium.launch()
const notes = []
for (const lang of ['en', 'ur', 'ar']) {
  const ctx = await browser.newContext({ viewport: { width: 1366, height: 800 }, locale: lang === 'en' ? 'en-US' : lang })
  await ctx.addInitScript((l) => { try { localStorage.setItem('lang', l) } catch (e) {} }, lang)
  const page = await ctx.newPage()
  page.on('pageerror', (e) => { notes.push(`[${lang}] page error: ${e.message}`) })
  page.on('console', (m) => { if (m.type() === 'error') notes.push(`[${lang}] console: ${m.text()}`) })
  await page.goto(base + '#/login')
  await settle(page)
  await page.screenshot({ path: `${out}/${lang}-00-login.png` })
  const dir = await page.evaluate(() => [document.documentElement.dir, document.documentElement.lang].join('/'))
  notes.push(`[${lang}] html dir/lang = ${dir}`)
  await page.locator('.login-form input').first().fill(process.env.FR_USER)
  await page.locator('.login-form input[type=password]').fill(process.env.FR_PASS)
  await page.locator('.login-button').first().click()
  await page.waitForURL((u) => !String(u).includes('/login'), { timeout: 15000 })
  await settle(page)
  for (const [name, hash] of pages) {
    await page.goto(base + hash)
    await settle(page)
    await page.screenshot({ path: `${out}/${lang}-${name}.png` })
  }
  // the language menu in the header
  await page.goto(base + '#/')
  await settle(page)
  await page.locator('.setting .el-dropdown').first().hover()
  await page.waitForTimeout(700)
  await page.screenshot({ path: `${out}/${lang}-08-language-menu.png` })
  await page.mouse.move(5, 5)
  await page.waitForTimeout(400)
  // the change-password window
  await page.locator('.info-form .el-button--danger').first().click()
  await page.waitForTimeout(800)
  await page.screenshot({ path: `${out}/${lang}-09-change-password.png` })
  await page.keyboard.press('Escape')
  // the add window of the address book
  await page.goto(base + '#/my/address_book')
  await settle(page)
  await page.locator('.el-button--danger').first().click()
  await page.waitForTimeout(800)
  await page.screenshot({ path: `${out}/${lang}-10-address-book-add.png` })
  await ctx.close()
}
await browser.close()
writeFileSync(`${out}/notes.txt`, notes.join('\n') + '\n')
console.log(notes.join('\n'))
console.log('screens done')
