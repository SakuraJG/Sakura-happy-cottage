import { useEffect, useState, type FormEvent } from 'react'
import { Flower2 } from 'lucide-react'
import { request } from '../../shared/api'
import type { User } from '../../shared/types'

type AuthMode = 'login' | 'register' | 'forgot'

export function AuthScreen({ onAuth }: { onAuth: (user: User) => void }) {
  const [mode, setMode] = useState<AuthMode>('login')
  const [account, setAccount] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [resetToken, setResetToken] = useState('')
  const [resetPassword, setResetPassword] = useState('')

  useEffect(() => {
    const parameters = new URLSearchParams(window.location.search)
    const verifyToken = parameters.get('verify_email')
    const resetToken = parameters.get('reset_password')
    if (verifyToken) {
      request<{ message: string }>('/api/auth/verify-email', { method: 'POST', body: JSON.stringify({ token: verifyToken }) })
        .then(payload => setMessage(payload.message)).catch(error => setMessage(error.message))
      window.history.replaceState({}, '', window.location.pathname)
    }
    if (resetToken) {
      setResetToken(resetToken)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [])

  async function submitReset(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const payload = await request<{ message: string }>('/api/auth/reset-password', {
        method: 'POST', body: JSON.stringify({ token: resetToken, new_password: resetPassword }),
      })
      setMessage(payload.message)
      setResetToken('')
      setResetPassword('')
    } catch (error) { setMessage((error as Error).message) } finally { setBusy(false) }
  }

  if (resetToken) return <section className="auth-shell">
    <header className="auth-header"><span className="brand-mark"><Flower2 size={17} /></span><span>Sakura 的快乐小屋</span></header>
    <main className="auth-grid">
      <div className="auth-copy"><p className="overline">账户安全</p><h1>重新设置，<br />回到你的小屋。</h1></div>
      <form className="auth-card" onSubmit={submitReset}><p className="overline">重置密码</p><h2>设置新密码</h2><label>新密码<input type="password" value={resetPassword} onChange={event => setResetPassword(event.target.value)} minLength={8} maxLength={72} required /></label>{message ? <p className="form-message">{message}</p> : null}<button className="button primary" disabled={busy}>{busy ? '处理中…' : '重置密码'}</button></form>
    </main>
  </section>

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      if (mode === 'forgot') {
        await request('/api/auth/forgot-password', { method: 'POST', body: JSON.stringify({ email }) })
        setMessage('如果邮箱已绑定，重置邮件将很快发送')
        return
      }
      const payload = await request<{ user: User }>(`/api/auth/${mode === 'login' ? 'login' : 'register'}`, {
        method: 'POST',
        body: JSON.stringify(mode === 'login' ? { account, password } : { account, username, password }),
      })
      onAuth(payload.user)
    } catch (error) {
      setMessage((error as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return <section className="auth-shell">
    <header className="auth-header"><span className="brand-mark"><Flower2 size={17} /></span><span>Sakura 的快乐小屋</span></header>
    <main className="auth-grid">
      <div className="auth-copy"><p className="overline">每日随记</p><h1>留下一笔，<br />让事情有迹可循。</h1><p>一个安静、可靠的私人空间，收好今天的想法，也和重要的人保持联系。</p></div>
      <form className="auth-card" key={mode} onSubmit={submit}>
        <p className="overline">{mode === 'login' ? '欢迎回来' : mode === 'register' ? '创建小屋账户' : '找回访问权限'}</p>
        <h2>{mode === 'login' ? '登录小屋' : mode === 'register' ? '加入小屋' : '重置密码'}</h2>
        {mode === 'register' ? <label>展示用户名<input value={username} onChange={event => setUsername(event.target.value)} minLength={2} maxLength={32} required /></label> : null}
        {mode === 'forgot' ? <label>邮箱<input type="email" value={email} onChange={event => setEmail(event.target.value)} required /></label> : <>
          <label>账户名<input value={account} onChange={event => setAccount(event.target.value)} required /></label>
          <label>密码<input type="password" value={password} onChange={event => setPassword(event.target.value)} minLength={mode === 'register' ? 8 : undefined} maxLength={72} required /></label>
        </>}
        {message ? <p className="form-message">{message}</p> : null}
        <button className="button primary" disabled={busy}>{busy ? '处理中…' : mode === 'login' ? '进入小屋' : mode === 'register' ? '创建账户' : '发送邮件'}</button>
        <div className="auth-links">
          {mode === 'login' ? <button type="button" onClick={() => setMode('forgot')}>忘记密码</button> : <span />}
          <button type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
            {mode === 'login' ? '创建账户' : mode === 'register' ? '已有账户？登录' : '返回登录'}
          </button>
        </div>
      </form>
    </main>
  </section>
}
