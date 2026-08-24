// 悬浮网页终端：xterm.js ⇄ WebSocket ⇄ PTY。
// 首次打开才懒加载 xterm 资源；收起保留会话，关闭断开并销毁。
(function () {
	const STORE_KEY = 'term-panel-geom';
	const DEFAULT_GEOM = { x: 60, y: 90, w: 640, h: 380 };

	function loadGeom() {
		try { return Object.assign({}, DEFAULT_GEOM, JSON.parse(localStorage.getItem(STORE_KEY) || '{}')); }
		catch (e) { return { ...DEFAULT_GEOM }; }
	}

	const Panel = {
		el: null,
		container: null,
		term: null,
		fit: null,
		ws: null,
		depsLoaded: false,
		connected: false,

		// 内嵌到指定容器（/admin 后台页），而非独立悬浮窗口
		attach(container) {
			this.container = container;
		},

		async ensureDeps() {
			if (this.depsLoaded) return;
			const load = src => new Promise((res, rej) => {
				const s = document.createElement('script');
				s.src = src; s.onload = res; s.onerror = () => rej(new Error('load failed: ' + src));
				document.head.appendChild(s);
			});
			await load('static/vendor/xterm.js?v=1');
			await load('static/vendor/xterm-addon-fit.js?v=1');
			this.depsLoaded = true;
		},

		createDOM() {
			if (this.el) return;
			this.el = document.createElement('div');
			this.el.className = 'term-panel';
			this.el.innerHTML = `
				<div class="term-titlebar">
					<span class="term-title">终端</span>
					<span class="term-actions">
						<button type="button" class="term-btn" data-act="collapse" title="收起（保留会话）">—</button>
						<button type="button" class="term-btn" data-act="close" title="关闭（结束会话）">×</button>
					</span>
				</div>
				<div class="term-body"></div>
				<div class="term-resize"></div>`;
			if (this.container) {
				// 内嵌模式（/admin）：占满容器，无拖动/拉伸/收起/关闭
				this.el.classList.add('term-embed');
				this.el.querySelector('[data-act="collapse"]').style.display = 'none';
				this.el.querySelector('[data-act="close"]').style.display = 'none';
				this.el.querySelector('.term-resize').style.display = 'none';
				this.container.appendChild(this.el);
			} else {
				document.body.appendChild(this.el);
				this.applyGeom(loadGeom());
				this.enableDrag();
				this.enableResize();
			}
			this.el.querySelector('[data-act="collapse"]').addEventListener('click', () => this.toggle());
			this.el.querySelector('[data-act="close"]').addEventListener('click', () => this.close());
		},

		applyGeom(g) {
			const maxX = window.innerWidth - 80, maxY = window.innerHeight - 40;
			this.el.style.left = Math.max(0, Math.min(g.x, maxX)) + 'px';
			this.el.style.top = Math.max(0, Math.min(g.y, maxY)) + 'px';
			this.el.style.width = g.w + 'px';
			this.el.style.height = g.h + 'px';
		},

		saveGeom() {
			const r = this.el.getBoundingClientRect();
			localStorage.setItem(STORE_KEY, JSON.stringify({ x: r.left, y: r.top, w: r.width, h: r.height }));
		},

		// 标题栏拖动（鼠标 + 触摸统一走 Pointer Events）
		enableDrag() {
			const bar = this.el.querySelector('.term-titlebar');
			bar.addEventListener('pointerdown', ev => {
				if (ev.target.closest('.term-btn')) return;
				const startX = ev.clientX, startY = ev.clientY;
				const rect = this.el.getBoundingClientRect();
				bar.setPointerCapture(ev.pointerId);
				const move = e => {
					this.el.style.left = Math.max(0, rect.left + e.clientX - startX) + 'px';
					this.el.style.top = Math.max(0, rect.top + e.clientY - startY) + 'px';
				};
				const up = () => {
					bar.removeEventListener('pointermove', move);
					bar.removeEventListener('pointerup', up);
					this.saveGeom();
				};
				bar.addEventListener('pointermove', move);
				bar.addEventListener('pointerup', up);
			});
		},

		// 右下角拉伸
		enableResize() {
			const handle = this.el.querySelector('.term-resize');
			handle.addEventListener('pointerdown', ev => {
				ev.preventDefault();
				const startX = ev.clientX, startY = ev.clientY;
				const rect = this.el.getBoundingClientRect();
				handle.setPointerCapture(ev.pointerId);
				const move = e => {
					this.el.style.width = Math.max(360, rect.width + e.clientX - startX) + 'px';
					this.el.style.height = Math.max(200, rect.height + e.clientY - startY) + 'px';
					if (this.fit) this.fit.fit();
				};
				const up = () => {
					handle.removeEventListener('pointermove', move);
					handle.removeEventListener('pointerup', up);
					this.saveGeom();
					if (this.fit) this.fit.fit();
				};
				handle.addEventListener('pointermove', move);
				handle.addEventListener('pointerup', up);
			});
		},

		async show() {
			this.createDOM();
			this.el.style.display = 'flex';
			await this.ensureDeps();
			if (this.term) { if (this.fit) this.fit.fit(); return; }
			if (await this.checkSession()) this.startSession();
			else this.renderLogin();
		},

		async checkSession() {
			try {
				const r = await fetch('/api/admin/session');
				return (await r.json()).authed === true;
			} catch (e) {
				return false;
			}
		},

		// 管理员登录表单（终端收在 admin 会话后面）
		renderLogin() {
			const body = this.el.querySelector('.term-body');
			body.innerHTML = `
				<div class="term-login">
					<div class="term-login-title">管理员登录</div>
					<input class="term-input" id="term-user" placeholder="用户名" autocomplete="username">
					<input class="term-input" id="term-pass" type="password" placeholder="密码" autocomplete="current-password">
					<button type="button" class="term-login-btn" id="term-login-btn">登录</button>
					<div class="term-login-err" id="term-login-err"></div>
				</div>`;
			const submit = async () => {
				const user = document.getElementById('term-user').value.trim();
				const pass = document.getElementById('term-pass').value;
				const err = document.getElementById('term-login-err');
				err.textContent = '';
				try {
					const r = await fetch('/api/admin/login', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ user, pass })
					});
					if (r.status === 204) {
						body.innerHTML = '';
						this.startSession();
						return;
					}
					if (r.status === 429) {
						let sec = 60;
						try {
							const d = await r.json();
							if (Number.isFinite(d.retry_after) && d.retry_after > 0) sec = Math.ceil(d.retry_after);
						} catch (e) { /* 解析失败用默认值 */ }
						err.textContent = `失败次数过多，请 ${sec} 秒后再试`;
						return;
					}
					err.textContent = r.status === 401 ? '用户名或密码错误' : `登录失败 (${r.status})`;
				} catch (e) {
					err.textContent = '网络错误';
				}
			};
			document.getElementById('term-login-btn').addEventListener('click', submit);
			document.getElementById('term-pass').addEventListener('keydown', e => { if (e.key === 'Enter') submit(); });
			document.getElementById('term-user').focus();
		},

		startSession() {
			const Terminal = window.Terminal, FitAddon = window.FitAddon.FitAddon;
			this.term = new Terminal({
				cursorBlink: true,
				fontSize: 12,
				fontFamily: 'Consolas, Menlo, monospace',
				theme: { background: '#141d2b' }
			});
			this.fit = new FitAddon();
			this.term.loadAddon(this.fit);
			this.term.open(this.el.querySelector('.term-body'));
			this.fit.fit();
			// 等字体就绪再重排一次，避免测量字体与前缀字库不一致导致的首行乱码/选中错位
			const fontsReady = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
			fontsReady.then(() => { if (this.fit) { this.fit.fit(); this.term.focus(); } });

			const proto = location.protocol === 'https:' ? 'wss://' : 'ws://';
			this.ws = new WebSocket(proto + location.host + '/ws/terminal');
			this.ws.binaryType = 'arraybuffer';

			this.ws.onopen = () => {
				this.connected = true;
				this.sendResize();
			};
			this.ws.onmessage = e => {
				if (e.data instanceof ArrayBuffer) this.term.write(new Uint8Array(e.data));
				else this.term.write(e.data);
			};
			this.ws.onclose = () => {
				this.connected = false;
				if (this.term) this.term.write('\r\n\x1b[33m[连接已断开]\x1b[0m\r\n');
			};

			this.term.onData(d => {
				if (this.connected) this.ws.send(new TextEncoder().encode(d));
			});
			this.term.onResize(({ cols, rows }) => this.sendResize(cols, rows));
			this.term.focus();
		},

		sendResize(cols, rows) {
			if (!this.ws || this.ws.readyState !== 1) return;
			if (cols === undefined && this.term) ({ cols, rows } = this.term);
			this.ws.send(JSON.stringify({ type: 'resize', cols, rows }));
		},

		toggle() {
			if (!this.el || this.el.style.display === 'none') this.show();
			else { this.el.style.display = 'none'; this.saveGeom(); }
		},

		close() {
			if (this.ws) { this.ws.onclose = null; this.ws.close(); }
			if (this.term) this.term.dispose();
			this.term = this.fit = this.ws = null;
			if (this.el) this.el.remove();
			this.el = null;
		},

		// 登出后：悬浮模式彻底关闭面板；内嵌模式回到登录表单
		async logoutNow() {
			try { await fetch('/api/admin/logout', { method: 'POST' }); } catch (e) { /* ignore */ }
			if (this.ws) { this.ws.onclose = null; this.ws.close(); this.ws = null; }
			if (this.term) { this.term.dispose(); this.term = this.fit = null; }
			if (this.el) {
				const body = this.el.querySelector('.term-body');
				if (body) body.innerHTML = '';
				if (this.container) this.renderLogin();
				else this.close();
			}
		}
	};

	window.TerminalPanel = Panel;
})();
