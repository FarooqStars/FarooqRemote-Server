<template>
  <!-- Farooq Remote (FA-020, owner 1 Nov 2026): "forgot password" - a 6-digit code to the account's e-mail, then a new
       password. The administrator's own account is not reset this way (FR-09 on the server). -->
  <div class="login-container">
    <div class="login-card fr-auth">
      <img src="@/assets/logo.png" alt="logo" class="login-logo"/>
      <h2 class="fr-title">{{ T('FrForgotTitle') }}</h2>
      <p class="fr-intro">{{ T('FrForgotIntro') }}</p>

      <div v-if="!open" class="fr-note">{{ T('FrNotOpen') }}</div>

      <el-form v-else-if="step === 1" label-position="top" class="login-form" @submit.prevent>
        <el-form-item :label="T('FrEmail')">
          <el-input v-model="form.email" type="email" autocomplete="email" dir="ltr" class="login-input"
                    @keyup.enter="sendCode"/>
        </el-form-item>
        <el-form-item :label="T('Captcha')">
          <el-input v-model="form.captcha" dir="ltr" class="login-input captcha-input" @keyup.enter="sendCode">
            <template #append>
              <img v-if="cap.b64" :src="cap.b64" @click="loadCaptcha" class="captcha" alt="captcha"/>
            </template>
          </el-input>
        </el-form-item>
        <el-button type="primary" class="login-button" :loading="busy" @click="sendCode">{{ T('FrSendCode') }}</el-button>
      </el-form>

      <el-form v-else label-position="top" class="login-form" @submit.prevent>
        <div class="fr-sent">{{ T('FrCodeSentTo') }} <b dir="ltr">{{ form.email }}</b></div>
        <el-form-item :label="T('FrCode')">
          <el-input v-model="form.code" maxlength="6" inputmode="numeric" autocomplete="one-time-code" dir="ltr"
                    class="login-input fr-code"/>
        </el-form-item>
        <el-form-item :label="T('FrNewPassword') + ' — ' + T('FrPasswordHint')">
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password"
                    class="login-input"/>
        </el-form-item>
        <el-form-item :label="T('ConfirmPassword')">
          <el-input v-model="form.confirm" type="password" show-password autocomplete="new-password"
                    class="login-input" @keyup.enter="save"/>
        </el-form-item>
        <el-button type="success" class="login-button" :loading="busy" @click="save">{{ T('FrSavePassword') }}</el-button>
        <a class="fr-link" @click="again">{{ T('FrSendAgain') }}</a>
      </el-form>

      <div class="fr-links"><a @click="router.push('/login')">{{ T('FrBackToLogin') }}</a></div>
      <div class="fr-langs">
        <a v-for="(v, k) in appStore.setting.langs" :key="k" :lang="k" :class="{ on: appStore.setting.lang === k }"
           @click="appStore.changeLang(k)">{{ v.name }}</a>
      </div>
    </div>
  </div>
</template>

<script setup>
  import { reactive, ref, onMounted } from 'vue'
  import { ElMessage } from 'element-plus'
  import { T } from '@/utils/i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { useAppStore } from '@/store/app'
  import { frOptions, frCaptcha, frPasswordCode, frPasswordReset } from '@/api/farooq'

  const route = useRoute()
  const router = useRouter()
  const appStore = useAppStore()
  const open = ref(true)
  const step = ref(1)
  const busy = ref(false)
  const cap = reactive({ id: '', b64: '' })
  const form = reactive({ email: '', captcha: '', code: '', password: '', confirm: '' })

  const loadCaptcha = async () => {
    form.captcha = ''
    const res = await frCaptcha().catch(_ => false)
    if (res && res.data && res.data.captcha) {
      cap.id = res.data.captcha.id
      cap.b64 = res.data.captcha.b64
    }
  }
  const sendCode = async () => {
    if (!form.email.trim()) {
      ElMessage.error(T('ParamRequired', { param: T('FrEmail') }))
      return
    }
    if (!form.captcha.trim()) {
      ElMessage.error(T('ParamRequired', { param: T('Captcha') }))
      return
    }
    busy.value = true
    const res = await frPasswordCode({
      email: form.email.trim(), captcha_id: cap.id, captcha: form.captcha.trim(), lang: appStore.setting.lang,
    }).catch(_ => false)
    busy.value = false
    if (!res) {
      loadCaptcha()
      return
    }
    ElMessage.success({ message: (res.data && res.data.message) || T('FrCodeSentTo'), duration: 7000 })
    step.value = 2
  }
  const save = async () => {
    if (!/^\d{6}$/.test(form.code.trim())) {
      ElMessage.error(T('ParamRequired', { param: T('FrCode') }))
      return
    }
    const n = [...form.password].length
    if (n < 8 || n > 32) {
      ElMessage.error(T('FrNewPassword') + ': ' + T('FrPasswordHint'))
      return
    }
    if (form.password !== form.confirm) {
      ElMessage.error(T('PasswordNotMatchConfirmPassword'))
      return
    }
    busy.value = true
    const res = await frPasswordReset({
      email: form.email.trim(), code: form.code.trim(), password: form.password, lang: appStore.setting.lang,
    }).catch(_ => false)
    busy.value = false
    if (!res) {
      return
    }
    ElMessage.success({ message: T('FrPasswordSaved'), duration: 6000 })
    router.push('/login')
  }
  const again = () => {
    step.value = 1
    form.code = ''
    loadCaptcha()
  }

  onMounted(async () => {
    const q = String(route.query.lang || '')
    if (q && appStore.setting.langs[q]) {
      appStore.changeLang(q)
    }
    if (route.query.email) {
      form.email = String(route.query.email)
    }
    const o = await frOptions().catch(_ => false)
    open.value = !!(o && o.data && o.data.signup)
    if (open.value) {
      loadCaptcha()
    }
  })
</script>

<style scoped lang="scss">
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: #2d3a4b;
  padding: 20px;
  box-sizing: border-box;
}

.login-card {
  width: 380px;
  max-width: 100%;
  background-color: #283342;
  padding: 36px 40px 28px;
  border-radius: 8px;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  text-align: center;
  color: #fff;
}

.login-logo {
  width: 72px;
  height: 72px;
  margin: 0 auto 14px;
  display: block;
}

.fr-title {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: bold;
}

.fr-intro {
  margin: 0 0 18px;
  font-size: 14px;
  line-height: 1.7;
  color: #c9d3df;
}

.fr-note {
  margin: 10px 0 20px;
  padding: 14px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.08);
  line-height: 1.7;
}

.fr-sent {
  margin: 0 0 14px;
  font-size: 14px;
  color: #c9d3df;
  overflow-wrap: anywhere;
}

.login-form {
  margin-bottom: 8px;
  text-align: start;
}

.login-input {
  width: 100%;
}

.fr-code ::v-deep(input) {
  letter-spacing: 6px;
  font-size: 18px;
  text-align: center;
}

.login-button {
  width: 100%;
  height: 40px;
  margin: 10px 0 12px;
}

.fr-link, .fr-links a {
  display: inline-block;
  cursor: pointer;
  color: #8fd3ff;
  font-size: 14px;
  margin: 4px 0;
}

.fr-links {
  margin-top: 10px;
}

.captcha {
  height: 30px;
  cursor: pointer;
  display: block;
}

.el-form-item {
  ::v-deep(.el-form-item__label) {
    color: #fff;
  }

  .el-input {
    ::v-deep(.el-input__wrapper) {
      border: 1px solid rgba(255, 255, 255, 0.1);
      background: transparent;
    }

    ::v-deep(input) {
      color: #fff;
    }
  }

  ::v-deep(.el-input-group__append) {
    padding: 0 6px;
    background: #fff;
  }
}
</style>
