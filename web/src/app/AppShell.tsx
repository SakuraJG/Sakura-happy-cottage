import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Gamepad2, House, LogOut, Settings, UserRound, UsersRound } from 'lucide-react'
import { request } from '../shared/api'
import type { SocialUser, User } from '../shared/types'
import { initial } from '../shared/format'

export type View = 'memos' | 'games' | 'people' | 'profile' | 'account'
export type SocialView = 'following' | 'friends'

export function AppShell({ user, view, contentKey, setView, openSocial, onLogout, children }: {
  user: User
  view: View
  contentKey: string
  setView: (view: View) => void
  openSocial: (view: SocialView) => void
  onLogout: () => void
  children: ReactNode
}) {
  const [menuOpen, setMenuOpen] = useState(false)
  const [social, setSocial] = useState<SocialUser | null>(null)
  const closeTimer = useRef<number | undefined>(undefined)
  const menuRef = useRef<HTMLDivElement>(null)

  const loadSocial = useCallback(() => {
    request<{ user: SocialUser }>(`/api/users/${user.uid}`)
      .then(payload => setSocial(payload.user))
      .catch(() => undefined)
  }, [user.uid])

  useEffect(() => {
    if (!menuOpen) return
    loadSocial()
    const close = (event: KeyboardEvent) => event.key === 'Escape' && setMenuOpen(false)
    const closeOutside = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false)
    }
    window.addEventListener('keydown', close)
    window.addEventListener('mousedown', closeOutside)
    return () => {
      window.removeEventListener('keydown', close)
      window.removeEventListener('mousedown', closeOutside)
    }
  }, [loadSocial, menuOpen])

  function openMenu() {
    window.clearTimeout(closeTimer.current)
    setMenuOpen(true)
  }

  function scheduleClose() {
    window.clearTimeout(closeTimer.current)
    closeTimer.current = window.setTimeout(() => setMenuOpen(false), 120)
  }

  function choose(action: () => void) {
    setMenuOpen(false)
    action()
  }

  return <div className="app-shell">
    <header className="topbar">
      <div className="topbar-inner">
        <button className="brand-button" onClick={() => setView('memos')} aria-label="返回我的备忘录">
          <span className="brand-mark"><House size={17} strokeWidth={1.8} /></span>
          <span>Sakura 的快乐小屋</span>
        </button>
        <nav aria-label="主要导航">
          <button className={view === 'memos' ? 'active' : ''} onClick={() => setView('memos')}>我的备忘录</button>
          <button className={view === 'games' ? 'active' : ''} onClick={() => setView('games')}><Gamepad2 size={15} />小游戏</button>
        </nav>
        <div className="profile-menu" ref={menuRef} onMouseEnter={openMenu} onMouseLeave={scheduleClose}>
          <button className="profile-trigger" aria-expanded={menuOpen} aria-haspopup="menu" onClick={openMenu}>
            <span className="avatar">{initial(user.username)}</span>
            <span className="profile-trigger-copy"><strong>{user.username}</strong><small>UID {user.uid}</small></span>
          </button>
          <div className={`profile-popover ${menuOpen ? 'open' : ''}`} role="menu" aria-hidden={!menuOpen}>
            <div className="popover-identity"><span className="avatar large-menu">{initial(user.username)}</span><div><strong>{user.username}</strong><span>@{user.account}</span></div></div>
            <div className="popover-stats">
              <button onClick={() => choose(() => openSocial('following'))}><strong>{social?.following_count ?? '—'}</strong><span>关注</span></button>
              <div><strong>{social?.follower_count ?? '—'}</strong><span>关注者</span></div>
              <button onClick={() => choose(() => openSocial('friends'))}><strong>{social?.friend_count ?? '—'}</strong><span>好友</span></button>
            </div>
            <div className="popover-actions">
              <button role="menuitem" onClick={() => choose(() => setView('profile'))}><UserRound size={17} /><span>个人空间<small>资料与公开信息</small></span></button>
              <button role="menuitem" onClick={() => choose(() => openSocial('friends'))}><UsersRound size={17} /><span>好友与关注<small>查找和管理关系</small></span></button>
              <button role="menuitem" onClick={() => choose(() => setView('account'))}><Settings size={17} /><span>账户与安全<small>邮箱、密码与系统设置</small></span></button>
            </div>
            <button className="popover-logout" role="menuitem" onClick={() => choose(onLogout)}><LogOut size={16} />退出登录</button>
          </div>
        </div>
      </div>
    </header>
    <main className="workspace"><div className="page-stage" key={contentKey}>{children}</div></main>
  </div>
}
