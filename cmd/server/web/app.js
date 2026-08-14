const state = {
  user: null,
  memos: [],
  filter: 'all',
  search: '',
  dateFilter: 'all',
  timeSort: 'desc',
  highlightMemoID: 0,
  maxUploadBytes: 0,
  smtpEnabled: false,
  registrationEnabled: true,
  emailConfirmationRequired: false,
  passwordRecoveryEnabled: false,
  resetToken: '',
  socialView: 'following',
  socialUsers: [],
  profileUser: null,
};

const $ = (selector) => document.querySelector(selector);

document.addEventListener('DOMContentLoaded', () => {
  setCurrentDate();
  wireAuthEvents();
  wireAppEvents();
  wireNavigation();
  bootstrap();
});

async function bootstrap() {
  await loadSettings();
  const params = new URLSearchParams(window.location.search);
  const verifyToken = params.get('verify_email');
  const resetToken = params.get('reset_password');
  if (verifyToken) {
    showAuth();
    await verifyEmail(verifyToken);
    clearActionURL();
  }
  if (resetToken) {
    state.resetToken = resetToken;
    showAuth();
    showAuthView('reset');
    return;
  }
  try {
    const { payload } = await requestJSON('/api/auth/me');
    enterApp(payload.user);
  } catch (_) {
    showAuth();
  }
}

function wireAuthEvents() {
  document.querySelectorAll('[data-auth-view]').forEach((button) => button.addEventListener('click', () => showAuthView(button.dataset.authView)));
  $('#show-forgot').addEventListener('click', () => showAuthView('forgot'));
  $('#login-form').addEventListener('submit', login);
  $('#register-form').addEventListener('submit', register);
  $('#forgot-form').addEventListener('submit', forgotPassword);
  $('#reset-form').addEventListener('submit', resetPassword);
}

function wireAppEvents() {
  $('#memo-form').addEventListener('submit', createMemo);
  $('#attachment-input').addEventListener('change', renderSelectedFiles);
  $('#refresh-button').addEventListener('click', () => loadMemos(true));
  $('#search-input').addEventListener('input', (event) => {
    state.search = event.target.value.trim().toLocaleLowerCase('zh-CN');
    renderMemos();
  });
  document.querySelectorAll('.tab').forEach((tab) => tab.addEventListener('click', () => selectFilter(tab)));
  $('#date-filter').addEventListener('change', updateDateFilter);
  $('#date-from').addEventListener('change', renderMemos);
  $('#date-to').addEventListener('change', renderMemos);
  $('#time-sort').addEventListener('change', (event) => { state.timeSort = event.target.value; renderMemos(); });
  $('#clear-filters').addEventListener('click', clearFilters);
  $('#edit-form').addEventListener('submit', saveEdit);
  $('#close-dialog').addEventListener('click', closeEditDialog);
  $('#cancel-edit').addEventListener('click', closeEditDialog);
  $('#account-button').addEventListener('click', () => navigateTo(`/profile/${state.user.uid}`));
  $('#menu-profile-button').addEventListener('click', () => navigateTo(`/profile/${state.user.uid}`));
  $('#menu-social-button').addEventListener('click', () => navigateTo('/people'));
  $('#menu-following-button').addEventListener('click', () => openSocialView('following'));
  $('#menu-friends-button').addEventListener('click', () => openSocialView('friends'));
  $('#menu-settings-button').addEventListener('click', openAccountDialog);
  $('#menu-logout-button').addEventListener('click', logout);
  $('#close-account').addEventListener('click', () => $('#account-dialog').close());
  $('#logout-button').addEventListener('click', logout);
  $('#email-form').addEventListener('submit', bindEmail);
  $('#password-form').addEventListener('submit', changePassword);
  $('#system-settings-form').addEventListener('submit', saveSystemSettings);
  $('#people-search-form').addEventListener('submit', searchUsers);
  document.querySelectorAll('[data-social-view]').forEach((tab) => tab.addEventListener('click', () => selectSocialView(tab.dataset.socialView)));
  $('#profile-form').addEventListener('submit', saveProfile);
  $('#profile-back').addEventListener('click', () => {
    if (window.history.length > 1) window.history.back();
    else navigateTo('/people');
  });
  document.querySelectorAll('dialog').forEach((dialog) => dialog.addEventListener('click', closeDialogFromBackdrop));
  document.addEventListener('keydown', (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !$('#memo-page').hidden && !$('#edit-dialog').open && !$('#account-dialog').open) {
      event.preventDefault();
      $('#memo-form').requestSubmit();
    }
  });
}

function wireNavigation() {
  document.querySelectorAll('[data-route]').forEach((link) => link.addEventListener('click', (event) => {
    const route = link.dataset.route;
    if (!route || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    navigateTo(route);
  }));
  window.addEventListener('popstate', renderRoute);
}

function renderRoute() {
  const pathname = window.location.pathname;
  const route = pathname === '/games' ? '/games' : (pathname === '/people' ? '/people' : (pathname.startsWith('/profile/') ? '/profile' : '/'));
  $('#memo-page').hidden = route !== '/';
  $('#games-page').hidden = route !== '/games';
  $('#people-page').hidden = route !== '/people';
  $('#profile-page').hidden = route !== '/profile';
  $('#refresh-button').hidden = route !== '/';
  document.querySelectorAll('.nav-link').forEach((link) => {
    const active = link.dataset.route === route;
    link.classList.toggle('active', active);
    if (active) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });
  if (route === '/people') loadSocialList();
  if (route === '/profile') loadProfile(pathname.slice('/profile/'.length));
  const titles = { '/games': '小游戏', '/people': '好友与关注', '/profile': '个人空间' };
  document.title = titles[route] ? `${titles[route]} · Sakura的快乐小屋` : 'Sakura的快乐小屋';
}

function navigateTo(route) {
  if (window.location.pathname !== route) window.history.pushState({}, '', route);
  renderRoute();
}

function closeDialogFromBackdrop(event) {
  const dialog = event.currentTarget;
  if (event.target !== dialog) return;
  const bounds = dialog.getBoundingClientRect();
  const inside = event.clientX >= bounds.left
    && event.clientX <= bounds.right
    && event.clientY >= bounds.top
    && event.clientY <= bounds.bottom;
  if (!inside) dialog.close();
}

function setCurrentDate() {
  const now = new Date();
  const weekday = new Intl.DateTimeFormat('zh-CN', { weekday: 'long' }).format(now);
  const longDate = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }).format(now);
  $('#weekday-label').textContent = `今天 · ${weekday}`;
  $('#date-label').textContent = longDate;
  $('#date-label').dateTime = now.toISOString();
  $('#composer-time').textContent = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' }).format(now);
  $('#auth-weekday').textContent = weekday;
  $('#auth-date').textContent = longDate;
}

async function loadSettings() {
  try {
    const { payload } = await requestJSON('/api/settings');
    state.maxUploadBytes = payload.max_upload_bytes || 0;
    state.smtpEnabled = Boolean(payload.smtp_enabled);
    state.registrationEnabled = Boolean(payload.registration_enabled);
    state.emailConfirmationRequired = Boolean(payload.email_confirmation_required);
    state.passwordRecoveryEnabled = Boolean(payload.password_recovery_enabled);
    $('#upload-limit').textContent = state.maxUploadBytes ? `最多 ${formatBytes(state.maxUploadBytes)}` : '';
    const registerTab = $('.auth-tab[data-auth-view="register"]');
    registerTab.hidden = !state.registrationEnabled;
    $('#show-forgot').hidden = !(state.passwordRecoveryEnabled && state.smtpEnabled);
    const emailButton = $('#email-form button[type="submit"]');
    emailButton.textContent = state.emailConfirmationRequired ? '发送确认邮件' : '绑定邮箱';
    emailButton.disabled = state.emailConfirmationRequired && !state.smtpEnabled;
    $('#smtp-state').textContent = state.emailConfirmationRequired
      ? (state.smtpEnabled ? '确认链接将发送到该邮箱' : '管理员尚未启用邮件服务')
      : '保存后直接绑定，无需邮件确认';
  } catch (_) {
    // Server-side validation remains authoritative when settings cannot load.
  }
}

function showAuth() {
  state.user = null;
  $('#main-app').hidden = true;
  $('#auth-screen').hidden = false;
  if (!state.resetToken) showAuthView('login');
}

function showAuthView(view) {
  if (view === 'register' && !state.registrationEnabled) view = 'login';
  if (view === 'forgot' && !(state.passwordRecoveryEnabled && state.smtpEnabled)) view = 'login';
  const views = ['login', 'register', 'forgot', 'reset'];
  views.forEach((name) => { $(`#${name}-form`).hidden = name !== view; });
  const simpleView = view === 'login' || view === 'register';
  $('#auth-tabs').hidden = !simpleView;
  document.querySelectorAll('.auth-tab').forEach((tab) => tab.classList.toggle('active', tab.dataset.authView === view));
  const titles = {
    login: ['欢迎回来', '登录后继续查看你的记录'],
    register: ['创建账号', '每个账号拥有独立的个人空间'],
    forgot: ['找回账号', '通过已确认的邮箱重置密码'],
    reset: ['重置密码', '设置一个新的安全密码'],
  };
  $('#auth-title').textContent = titles[view][0];
  $('#auth-subtitle').textContent = titles[view][1];
  const firstInput = $(`#${view}-form input`);
  if (firstInput) window.setTimeout(() => firstInput.focus(), 0);
}

function enterApp(user) {
  state.user = user;
  state.resetToken = '';
  $('#auth-screen').hidden = true;
  $('#main-app').hidden = false;
  renderUser();
  renderRoute();
  loadMemos();
  refreshSocialCounts().catch(() => {});
}

function renderUser() {
  if (!state.user) return;
  const initial = [...state.user.username][0] || 'U';
  $('#account-avatar').textContent = initial;
  $('#menu-avatar').textContent = initial;
  $('#dialog-avatar').textContent = initial;
  $('#account-name').textContent = state.user.username;
  $('#menu-username').textContent = state.user.username;
  $('#menu-uid').textContent = `UID ${state.user.uid}`;
  $('#dialog-username').textContent = state.user.username;
  const verified = state.user.email_verified;
  $('#email-status').textContent = verified ? state.user.email : '尚未绑定已确认邮箱';
  $('#email-badge').textContent = verified ? '已确认' : '未绑定';
  $('#email-badge').classList.toggle('verified', verified);
  $('#bind-email').value = state.user.email || '';
  const isAdmin = state.user.role === 'admin';
  $('#menu-admin-storage').hidden = !isAdmin;
  $('#admin-section').hidden = !isAdmin;
  $('#admin-password-warning').hidden = !(isAdmin && state.user.must_change_password);
}

async function login(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ account: $('#login-account').value.trim(), password: $('#login-password').value }),
    });
    form.reset();
    enterApp(payload.user);
    showToast('登录成功');
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function register(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const password = $('#register-password').value;
  if (password !== $('#register-confirm').value) {
    showToast('两次输入的密码不一致', true);
    return;
  }
  const button = form.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ account: $('#register-account').value.trim(), username: $('#register-username').value.trim(), password }),
    });
    form.reset();
    enterApp(payload.user);
    showToast('账号已创建');
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function forgotPassword(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/auth/forgot-password', {
      method: 'POST',
      body: JSON.stringify({ email: $('#forgot-email').value.trim() }),
    });
    form.reset();
    showAuthView('login');
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function resetPassword(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const password = $('#reset-password').value;
  if (password !== $('#reset-confirm').value) {
    showToast('两次输入的密码不一致', true);
    return;
  }
  const button = form.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/auth/reset-password', {
      method: 'POST',
      body: JSON.stringify({ token: state.resetToken, new_password: password }),
    });
    state.resetToken = '';
    clearActionURL();
    form.reset();
    showAuthView('login');
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function verifyEmail(token) {
  try {
    const { payload } = await requestJSON('/api/auth/verify-email', {
      method: 'POST',
      body: JSON.stringify({ token }),
    });
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  }
}

async function logout() {
  try {
    await requestJSON('/api/auth/logout', { method: 'POST' });
  } catch (_) {
    // Local session state is cleared even if the server session already expired.
  }
  if ($('#account-dialog').open) $('#account-dialog').close();
  state.memos = [];
  showAuth();
  showToast('已退出登录');
}

function openAccountDialog() {
  renderUser();
  $('#account-dialog').showModal();
  if (state.user?.role === 'admin') loadAdminSystem();
}

async function loadAdminSystem() {
  try {
    const { payload } = await requestJSON('/api/admin/system');
    $('#system-users').textContent = payload.users;
    $('#system-memos').textContent = payload.memos;
    $('#system-uptime').textContent = formatDuration(payload.uptime_seconds);
    populateSystemSettings(payload.settings);
  } catch (error) {
    showToast(error.message, true);
  }
}

function populateSystemSettings(settings) {
  $('#setting-registration').checked = Boolean(settings.registration_enabled);
  $('#setting-email-confirm').checked = Boolean(settings.email_confirmation_required);
  $('#setting-recovery').checked = Boolean(settings.password_recovery_enabled);
  $('#setting-smtp-enabled').checked = Boolean(settings.smtp_enabled);
  $('#setting-public-url').value = settings.public_url || '';
  $('#setting-smtp-host').value = settings.smtp_host || '';
  $('#setting-smtp-port').value = settings.smtp_port || 587;
  $('#setting-smtp-encryption').value = settings.smtp_encryption || 'starttls';
  $('#setting-smtp-username').value = settings.smtp_username || '';
  $('#setting-smtp-password').value = '';
  $('#setting-smtp-password').placeholder = settings.smtp_password_set ? '已保存，留空则保持不变' : '填写密码或授权码';
  $('#setting-smtp-from-name').value = settings.smtp_from_name || '';
  $('#setting-smtp-from-email').value = settings.smtp_from_email || '';
  $('#system-save-state').textContent = settings.smtp_password_set ? 'SMTP 密码已安全保存，不会回显' : '尚未保存 SMTP 密码';
}

async function saveSystemSettings(event) {
  event.preventDefault();
  const button = event.currentTarget.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/admin/system', {
      method: 'PUT',
      body: JSON.stringify({
        registration_enabled: $('#setting-registration').checked,
        email_confirmation_required: $('#setting-email-confirm').checked,
        password_recovery_enabled: $('#setting-recovery').checked,
        public_url: $('#setting-public-url').value.trim(),
        smtp_enabled: $('#setting-smtp-enabled').checked,
        smtp_host: $('#setting-smtp-host').value.trim(),
        smtp_port: Number($('#setting-smtp-port').value),
        smtp_encryption: $('#setting-smtp-encryption').value,
        smtp_username: $('#setting-smtp-username').value.trim(),
        smtp_password: $('#setting-smtp-password').value,
        smtp_from_name: $('#setting-smtp-from-name').value.trim(),
        smtp_from_email: $('#setting-smtp-from-email').value.trim(),
      }),
    });
    populateSystemSettings(payload.settings);
    await loadSettings();
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function bindEmail(event) {
  event.preventDefault();
  const button = event.currentTarget.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/account/email', {
      method: 'POST',
      body: JSON.stringify({ email: $('#bind-email').value.trim(), password: $('#email-password').value }),
    });
    $('#email-password').value = '';
    if (payload.email) {
      state.user.email = payload.email;
      state.user.email_verified = Boolean(payload.email_verified);
      renderUser();
    }
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function changePassword(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const newPassword = $('#new-password').value;
  if (newPassword !== $('#confirm-password').value) {
    showToast('两次输入的新密码不一致', true);
    return;
  }
  const button = form.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/account/password', {
      method: 'POST',
      body: JSON.stringify({ current_password: $('#current-password').value, new_password: newPassword }),
    });
    form.reset();
    state.user.must_change_password = false;
    renderUser();
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function loadSocialList() {
  const list = $('#people-list');
  $('#people-loading').hidden = false;
  list.innerHTML = '';
  $('#people-empty').hidden = true;
  try {
    const [followingResponse, friendsResponse] = await Promise.all([
      requestJSON('/api/social/following'),
      requestJSON('/api/social/friends'),
    ]);
    const following = followingResponse.payload.users || [];
    const friends = friendsResponse.payload.users || [];
    setSocialCounts(following.length, friends.length);
    state.socialUsers = state.socialView === 'friends' ? friends : following;
    $('#people-result-label').textContent = state.socialView === 'friends' ? '好友' : '我的关注';
    renderPeople();
  } catch (error) {
    showToast(error.message, true);
  } finally {
    $('#people-loading').hidden = true;
  }
}

async function refreshSocialCounts() {
  const [followingResponse, friendsResponse] = await Promise.all([
    requestJSON('/api/social/following'),
    requestJSON('/api/social/friends'),
  ]);
  const followingCount = (followingResponse.payload.users || []).length;
  const friendCount = (friendsResponse.payload.users || []).length;
  setSocialCounts(followingCount, friendCount);
}

function setSocialCounts(followingCount, friendCount) {
  $('#following-count').textContent = followingCount;
  $('#friend-count').textContent = friendCount;
  $('#menu-following-count').textContent = followingCount;
  $('#menu-friend-count').textContent = friendCount;
}

function openSocialView(view) {
  state.socialView = view;
  document.querySelectorAll('[data-social-view]').forEach((tab) => {
    const active = tab.dataset.socialView === view;
    tab.classList.toggle('active', active);
    tab.setAttribute('aria-selected', String(active));
  });
  $('#people-search-input').value = '';
  navigateTo('/people');
}

function selectSocialView(view) {
  state.socialView = view;
  document.querySelectorAll('[data-social-view]').forEach((tab) => {
    const active = tab.dataset.socialView === view;
    tab.classList.toggle('active', active);
    tab.setAttribute('aria-selected', String(active));
  });
  $('#people-search-input').value = '';
  loadSocialList();
}

async function searchUsers(event) {
  event.preventDefault();
  const query = $('#people-search-input').value.trim();
  if (!query) return;
  const button = event.currentTarget.querySelector('button[type="submit"]');
  setBusy(button, true);
  $('#people-loading').hidden = false;
  try {
    const { payload } = await requestJSON(`/api/users/search?q=${encodeURIComponent(query)}`);
    state.socialUsers = payload.users || [];
    $('#people-result-label').textContent = `“${query}”的搜索结果`;
    renderPeople(true);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
    $('#people-loading').hidden = true;
  }
}

function renderPeople(isSearch = false) {
  $('#people-list').innerHTML = state.socialUsers.map(personTemplate).join('');
  const empty = $('#people-empty');
  empty.hidden = state.socialUsers.length > 0;
  empty.querySelector('h3').textContent = isSearch ? '没有找到用户' : (state.socialView === 'friends' ? '还没有好友' : '还没有关注');
  empty.querySelector('p').textContent = isSearch ? '检查 UID 或换一个用户名试试。' : (state.socialView === 'friends' ? '双方互相关注后会自动成为好友。' : '通过 UID 或用户名找到其他用户。');
  wirePersonActions();
}

function personTemplate(user) {
  const initial = escapeHTML([...user.username][0] || 'U');
  let relation = '';
  if (user.is_self) relation = '<span class="relation-badge">自己</span>';
  else if (user.friend) relation = '<span class="relation-badge friend">好友</span>';
  else if (user.followed_by) relation = '<span class="relation-badge">关注了你</span>';
  const action = user.is_self ? '' : `<button class="${user.following ? 'secondary-button' : 'primary-button'} compact-button" data-follow-uid="${user.uid}" data-following="${user.following}" type="button">${user.following ? '已关注' : '关注'}</button>`;
  return `<article class="person-row"><button class="person-main" data-profile-uid="${user.uid}" type="button"><span class="account-avatar person-avatar">${initial}</span><span class="person-copy"><strong>${escapeHTML(user.username)}</strong><span>UID ${user.uid}${user.bio ? ` · ${escapeHTML(user.bio)}` : ''}</span></span>${relation}</button>${action}</article>`;
}

function wirePersonActions() {
  document.querySelectorAll('[data-profile-uid]').forEach((button) => button.addEventListener('click', () => navigateTo(`/profile/${button.dataset.profileUid}`)));
  document.querySelectorAll('[data-follow-uid]').forEach((button) => button.addEventListener('click', () => toggleFollow(button.dataset.followUid, button.dataset.following === 'true')));
}

async function loadProfile(uid) {
  if (!/^\d+$/.test(uid)) {
    navigateTo('/people');
    return;
  }
  $('#profile-loading').hidden = false;
  $('#profile-content').hidden = true;
  try {
    const { payload } = await requestJSON(`/api/users/${uid}`);
    state.profileUser = payload.user;
    renderProfile();
    $('#profile-content').hidden = false;
  } catch (error) {
    showToast(error.message, true);
    if (error.status === 404) navigateTo('/people');
  } finally {
    $('#profile-loading').hidden = true;
  }
}

function renderProfile() {
  const user = state.profileUser;
  if (!user) return;
  $('#profile-avatar').textContent = [...user.username][0] || 'U';
  $('#profile-username').textContent = user.username;
  $('#profile-uid').textContent = `UID ${user.uid}`;
  $('#profile-following-count').textContent = user.following_count;
  $('#profile-follower-count').textContent = user.follower_count;
  $('#profile-friend-count').textContent = user.friend_count;
  $('#profile-created-at').textContent = `${formatMonth(user.created_at)} 加入`;
  $('#profile-bio').textContent = user.bio || '这个人还没有填写个人简介。';
  $('#profile-relation').textContent = user.is_self ? '我的个人空间' : (user.friend ? '好友' : (user.followed_by ? '关注了你' : '个人空间'));
  $('#own-profile-section').hidden = !user.is_self;
  $('#profile-actions').innerHTML = user.is_self
    ? '<div class="profile-action-group"><button class="secondary-button" data-open-people type="button">好友与关注</button><button class="secondary-button" data-open-settings type="button">账户与安全</button><button class="danger-button" data-profile-logout type="button">退出登录</button></div>'
    : `<button class="${user.following ? 'secondary-button' : 'primary-button'}" data-profile-follow type="button">${user.following ? '已关注' : '关注'}</button>`;
  if (user.is_self) {
    setSocialCounts(user.following_count, user.friend_count);
    $('#profile-edit-username').value = state.user.username;
    $('#profile-edit-bio').value = state.user.bio || '';
    $('#profile-account').textContent = state.user.account;
    $('#profile-email').textContent = state.user.email_verified ? state.user.email : '尚未绑定';
    $('[data-open-people]').addEventListener('click', () => navigateTo('/people'));
    $('[data-open-settings]').addEventListener('click', openAccountDialog);
    $('[data-profile-logout]').addEventListener('click', logout);
  } else {
    $('[data-profile-follow]').addEventListener('click', () => toggleFollow(user.uid, user.following, true));
  }
}

async function toggleFollow(uid, following, onProfile = false) {
  try {
    const { payload } = await requestJSON(`/api/users/${uid}/follow`, { method: following ? 'DELETE' : 'POST' });
    showToast(payload.message);
    if (onProfile) {
      state.profileUser = payload.user;
      renderProfile();
      await refreshSocialCounts();
    } else if ($('#people-search-input').value.trim()) {
      const index = state.socialUsers.findIndex((user) => user.uid === Number(uid));
      if (index !== -1) state.socialUsers[index] = payload.user;
      renderPeople(true);
      await refreshSocialCounts();
    } else {
      loadSocialList();
    }
  } catch (error) {
    showToast(error.message, true);
  }
}

async function saveProfile(event) {
  event.preventDefault();
  const button = event.currentTarget.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    const { payload } = await requestJSON('/api/account/profile', {
      method: 'PATCH',
      body: JSON.stringify({ username: $('#profile-edit-username').value.trim(), bio: $('#profile-edit-bio').value.trim() }),
    });
    state.user = payload.user;
    renderUser();
    await loadProfile(state.user.uid);
    showToast(payload.message);
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function loadMemos(showSuccess = false) {
  if (showSuccess) $('#refresh-button').classList.add('spinning');
  try {
    const { payload } = await requestJSON('/api/memos');
    state.memos = payload.memos || [];
    renderMemos();
    if (showSuccess) showToast('记录已刷新');
  } catch (error) {
    if (error.status === 401) {
      showAuth();
      return;
    }
    showToast(error.message, true);
  } finally {
    $('#loading-state').hidden = true;
    $('#refresh-button').classList.remove('spinning');
  }
}

async function createMemo(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector('button[type="submit"]');
  const data = new FormData(form);
  data.append('status', 'open');
  const files = [...$('#attachment-input').files];
  const totalBytes = files.reduce((total, file) => total + file.size, 0);
  if (state.maxUploadBytes && totalBytes > state.maxUploadBytes) {
    showToast(`附件总大小不能超过 ${formatBytes(state.maxUploadBytes)}`, true);
    return;
  }
  setBusy(button, true);
  try {
    const response = await fetch('/api/memos', { method: 'POST', body: data });
    const payload = await response.json();
    if (!response.ok) throw apiError(response, payload);
    state.memos.unshift(payload);
    state.highlightMemoID = payload.id;
    form.reset();
    renderSelectedFiles();
    renderMemos();
    const composer = $('.composer');
    composer.classList.remove('saved');
    void composer.offsetWidth;
    composer.classList.add('saved');
    window.setTimeout(() => composer.classList.remove('saved'), 700);
    $('#title-input').focus();
    showToast('记录已保存');
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function updateMemo(memo, changes) {
  const { payload } = await requestJSON(`/api/memos/${memo.id}`, {
    method: 'PATCH',
    body: JSON.stringify({
      title: changes.title ?? memo.title,
      description: changes.description ?? memo.description,
      status: changes.status ?? memo.status,
    }),
  });
  const index = state.memos.findIndex((item) => item.id === memo.id);
  if (index !== -1) state.memos[index] = payload;
  renderMemos();
  return payload;
}

async function toggleMemo(memo) {
  if (!memo) return;
  const status = memo.status === 'done' ? 'open' : 'done';
  try {
    await updateMemo(memo, { status });
    showToast(status === 'done' ? '已标记完成' : '已恢复进行中');
  } catch (error) {
    showToast(error.message, true);
  }
}

function openEditDialog(memo) {
  if (!memo) return;
  $('#edit-id').value = memo.id;
  $('#edit-title').value = memo.title;
  $('#edit-description').value = memo.description;
  $('#edit-dialog').showModal();
  $('#edit-title').focus();
}

function closeEditDialog() {
  $('#edit-dialog').close();
}

async function saveEdit(event) {
  event.preventDefault();
  const memo = state.memos.find((item) => item.id === Number($('#edit-id').value));
  if (!memo) return;
  const button = event.currentTarget.querySelector('button[type="submit"]');
  setBusy(button, true);
  try {
    await updateMemo(memo, { title: $('#edit-title').value.trim(), description: $('#edit-description').value.trim() });
    closeEditDialog();
    showToast('修改已保存');
  } catch (error) {
    showToast(error.message, true);
  } finally {
    setBusy(button, false);
  }
}

async function deleteMemo(memo) {
  if (!memo || !window.confirm(`确定删除“${memo.title}”吗？附件也会一并删除。`)) return;
  try {
    await requestJSON(`/api/memos/${memo.id}`, { method: 'DELETE' });
    state.memos = state.memos.filter((item) => item.id !== memo.id);
    renderMemos();
    showToast('记录已删除');
  } catch (error) {
    showToast(error.message, true);
  }
}

function selectFilter(selected) {
  state.filter = selected.dataset.filter;
  document.querySelectorAll('.tab').forEach((tab) => {
    const active = tab === selected;
    tab.classList.toggle('active', active);
    tab.setAttribute('aria-selected', String(active));
  });
  renderMemos();
}

function updateDateFilter(event) {
  state.dateFilter = event.target.value;
  $('#custom-date-range').hidden = state.dateFilter !== 'custom';
  renderMemos();
}

function clearFilters() {
  state.filter = 'all';
  state.search = '';
  state.dateFilter = 'all';
  state.timeSort = 'desc';
  $('#search-input').value = '';
  $('#date-filter').value = 'all';
  $('#time-sort').value = 'desc';
  $('#date-from').value = '';
  $('#date-to').value = '';
  $('#custom-date-range').hidden = true;
  document.querySelectorAll('.tab').forEach((tab) => {
    const active = tab.dataset.filter === 'all';
    tab.classList.toggle('active', active);
    tab.setAttribute('aria-selected', String(active));
  });
  renderMemos();
}

function renderMemos() {
  const openCount = state.memos.filter((memo) => memo.status === 'open').length;
  const doneCount = state.memos.length - openCount;
  $('#count-all').textContent = state.memos.length;
  $('#count-open').textContent = openCount;
  $('#count-done').textContent = doneCount;
  const visible = state.memos.filter((memo) => {
    const matchesStatus = state.filter === 'all' || memo.status === state.filter;
    const haystack = `${memo.title} ${memo.description}`.toLocaleLowerCase('zh-CN');
    return matchesStatus && (!state.search || haystack.includes(state.search)) && matchesDate(memo.created_at);
  }).sort((left, right) => {
    const difference = new Date(left.created_at).getTime() - new Date(right.created_at).getTime();
    return state.timeSort === 'asc' ? difference : -difference;
  });
  $('#clear-filters').hidden = state.filter === 'all' && !state.search && state.dateFilter === 'all' && state.timeSort === 'desc';
  $('#memo-list').innerHTML = visible.map(memoTemplate).join('');
  const empty = $('#empty-state');
  empty.hidden = visible.length > 0;
  empty.querySelector('h3').textContent = state.memos.length === 0 ? '还没有记录' : '没有匹配的记录';
  empty.querySelector('p').textContent = state.memos.length === 0 ? '先写下今天的第一件事。' : '换一个关键词或状态试试。';
  document.querySelectorAll('[data-toggle]').forEach((button) => button.addEventListener('click', () => toggleMemo(findMemo(button.dataset.toggle))));
  document.querySelectorAll('[data-edit]').forEach((button) => button.addEventListener('click', () => openEditDialog(findMemo(button.dataset.edit))));
  document.querySelectorAll('[data-delete]').forEach((button) => button.addEventListener('click', () => deleteMemo(findMemo(button.dataset.delete))));
  if (state.highlightMemoID) {
    const highlighted = document.querySelector(`[data-memo-id="${state.highlightMemoID}"]`);
    if (highlighted) window.setTimeout(() => highlighted.classList.remove('is-new'), 750);
    state.highlightMemoID = 0;
  }
}

function matchesDate(value) {
  if (state.dateFilter === 'all') return true;
  const created = new Date(value);
  const now = new Date();
  let from = null;
  let to = null;
  if (state.dateFilter === 'today') {
    from = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  } else if (state.dateFilter === '7days' || state.dateFilter === '30days') {
    const days = state.dateFilter === '7days' ? 6 : 29;
    from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - days);
  } else if (state.dateFilter === 'custom') {
    if ($('#date-from').value) from = new Date(`${$('#date-from').value}T00:00:00`);
    if ($('#date-to').value) to = new Date(`${$('#date-to').value}T23:59:59.999`);
  }
  return (!from || created >= from) && (!to || created <= to);
}

function memoTemplate(memo) {
  const done = memo.status === 'done';
  const description = memo.description ? escapeHTML(memo.description) : '<span class="muted-copy">无补充描述</span>';
  const attachments = (memo.attachments || []).map(attachmentTemplate).join('');
  const completedTime = done && memo.completed_at ? `<span class="completed-time">完成 ${formatDateTime(memo.completed_at)}</span>` : '';
  const isNew = memo.id === state.highlightMemoID;
  return `<article class="memo-card${done ? ' done' : ''}${isNew ? ' is-new' : ''}" data-memo-id="${memo.id}"><div class="memo-top"><h3 class="memo-title">${escapeHTML(memo.title)}</h3><button class="status-toggle" data-toggle="${memo.id}" type="button" title="${done ? '恢复进行中' : '标记完成'}" aria-label="${done ? '恢复进行中' : '标记完成'}">✓</button></div><p class="memo-description">${description}</p>${attachments ? `<div class="attachment-list">${attachments}</div>` : ''}<div class="memo-bottom"><div class="memo-time"><time datetime="${escapeAttr(memo.created_at)}">记录 ${formatDateTime(memo.created_at)}</time>${completedTime}</div><div class="card-actions"><button class="small-action" data-edit="${memo.id}" type="button" title="编辑" aria-label="编辑">✎</button><button class="small-action delete" data-delete="${memo.id}" type="button" title="删除" aria-label="删除">⌫</button></div></div></article>`;
}

function attachmentTemplate(file) {
  const name = escapeHTML(file.original_name);
  const title = escapeAttr(`${file.original_name} · ${formatBytes(file.size)}`);
  const url = `/api/attachments/${file.id}`;
  if (['image/jpeg', 'image/png', 'image/gif', 'image/webp'].includes(String(file.content_type).toLowerCase())) {
    return `<a class="attachment-image" href="${url}?inline=1" target="_blank" rel="noopener" title="${title}"><img src="${url}?inline=1" alt="${escapeAttr(file.original_name)}" loading="lazy"></a>`;
  }
  return `<a class="attachment-link" href="${url}" title="${title}">↓ ${name}</a>`;
}

function renderSelectedFiles() {
  const files = [...$('#attachment-input').files];
  $('#selected-files').innerHTML = files.map((file) => `<span class="selected-file" title="${escapeAttr(file.name)}">${escapeHTML(file.name)} · ${formatBytes(file.size)}</span>`).join('');
}

async function requestJSON(url, options = {}) {
  const headers = { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) };
  const response = await fetch(url, { ...options, headers });
  const payload = response.status === 204 ? {} : await response.json().catch(() => ({}));
  if (!response.ok) throw apiError(response, payload);
  return { response, payload };
}

function apiError(response, payload) {
  const error = new Error(payload.error || '请求处理失败');
  error.status = response.status;
  return error;
}

function setBusy(button, busy) {
  button.disabled = busy;
  button.setAttribute('aria-busy', String(busy));
}

function clearActionURL() {
  window.history.replaceState({}, '', window.location.pathname);
}

function findMemo(id) {
  return state.memos.find((memo) => memo.id === Number(id));
}

function formatDateTime(value) {
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}

function formatMonth(value) {
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long' }).format(new Date(value));
}

function formatBytes(bytes) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / (1024 ** index);
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatDuration(seconds) {
  if (seconds < 60) return `${seconds} 秒`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时`;
  return `${Math.floor(seconds / 86400)} 天`;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[character]);
}

function escapeAttr(value) {
  return escapeHTML(value);
}

let toastTimer;
function showToast(message, isError = false) {
  const node = $('#toast');
  node.textContent = message;
  node.className = 'toast';
  void node.offsetWidth;
  node.className = `toast show${isError ? ' error' : ''}`;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { node.className = 'toast'; }, 3200);
}
