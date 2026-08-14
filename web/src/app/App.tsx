import { startTransition, useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { ShieldCheck } from 'lucide-react'
import { request } from '../shared/api'
import type { User } from '../shared/types'
import { AuthScreen } from '../features/auth/AuthScreen'
import { AccountPanel } from '../features/account/AccountPanel'
import { GamesPage } from '../features/games/GamesPage'
import { MemoPage } from '../features/memos/MemoPage'
import { PeoplePage } from '../features/social/PeoplePage'
import { ProfilePage } from '../features/social/ProfilePage'
import { AppShell, type SocialView, type View } from './AppShell'

export function App() {
  const [user, setUser] = useState<User | null>(null)
  const [checkingSession, setCheckingSession] = useState(true)
  const [view, setView] = useState<View>('memos')
  const [socialView, setSocialView] = useState<SocialView>('following')
  const [profileUID, setProfileUID] = useState<number | null>(null)
  const [toast, setToast] = useState<{ id: number; message: string } | null>(null)
  const toastTimer = useRef<number | undefined>(undefined)

  useEffect(() => {
    request<{ user: User }>('/api/auth/me')
      .then(payload => setUser(payload.user))
      .catch(() => undefined)
      .finally(() => setCheckingSession(false))
  }, [])

  const notify = useCallback((message: string) => {
    window.clearTimeout(toastTimer.current)
    setToast({ id: Date.now(), message })
    toastTimer.current = window.setTimeout(() => setToast(null), 3200)
  }, [])

  function navigate(next: View) {
    startTransition(() => {
      setProfileUID(null)
      setView(next)
    })
  }

  function openSocial(next: SocialView) {
    setSocialView(next)
    navigate('people')
  }

  function openProfile(uid: number) {
    if (uid === user?.uid) {
      navigate('profile')
      return
    }
    startTransition(() => setProfileUID(uid))
  }

  async function logout() {
    await request('/api/auth/logout', { method: 'POST' }).catch(() => undefined)
    setUser(null)
    setView('memos')
  }

  if (checkingSession) return <div className="app-loading"><span className="brand-mark">✿</span><p>正在打开小屋</p></div>
  if (!user) return <AuthScreen onAuth={setUser} />

  let content: ReactNode
  let contentKey = profileUID ? `profile-${profileUID}` : view
  if (profileUID) content = <ProfilePage uid={profileUID} user={user} onUser={setUser} onToast={notify} onBack={() => setProfileUID(null)} onSocial={openSocial} onAccount={() => navigate('account')} />
  else if (view === 'memos') content = <MemoPage onToast={notify} />
  else if (view === 'people') content = <PeoplePage initialTab={socialView} onToast={notify} onProfile={openProfile} />
  else if (view === 'profile') content = <ProfilePage uid={user.uid} user={user} onUser={setUser} onToast={notify} onBack={() => navigate('memos')} onSocial={openSocial} onAccount={() => navigate('account')} />
  else if (view === 'account') content = <div className="account-page"><section className="page-heading"><div><p className="overline">账户</p><h1>账户与安全</h1><p className="heading-note">管理登录方式、密码与小屋的系统配置。</p></div><ShieldCheck size={30} strokeWidth={1.4} /></section><AccountPanel user={user} onToast={notify} /></div>
  else content = <GamesPage />

  return <>
    <AppShell user={user} view={view} contentKey={contentKey} setView={navigate} openSocial={openSocial} onLogout={logout}>{content}</AppShell>
    {toast ? <div className="toast" key={toast.id} role="status"><span>✓</span>{toast.message}</div> : null}
  </>
}
