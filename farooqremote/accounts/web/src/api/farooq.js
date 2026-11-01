// Farooq Remote (FA-020): sign-up and "forgot password" by an e-mail code (server: http/controller/admin/farooq_signup.go)
import request from '@/utils/request'

export const frOptions = () => request({ url: '/fr/options', method: 'get' })
export const frCaptcha = () => request({ url: '/fr/captcha', method: 'get' })
export const frSignupCode = (data) => request({ url: '/fr/signup/code', method: 'post', data })
export const frSignupFinish = (data) => request({ url: '/fr/signup/finish', method: 'post', data })
export const frPasswordCode = (data) => request({ url: '/fr/password/code', method: 'post', data })
export const frPasswordReset = (data) => request({ url: '/fr/password/reset', method: 'post', data })

// the farooqmughal.net pages in the page language (Urdu and Arabic have their own; others use English)
export const frSitePage = (lang, page) => {
  const pre = lang === 'ur' ? 'ur/' : lang === 'ar' ? 'ar/' : ''
  return `https://www.farooqmughal.net/${pre}farooqremote/${page}/`
}
