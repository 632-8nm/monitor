(function () {
	const UI = {
		formatUptime(seconds) {
			const d = Math.floor(seconds / (3600 * 24));
			const h = Math.floor((seconds % (3600 * 24)) / 3600);
			const m = Math.floor((seconds % 3600) / 60);
			return `${d}天 ${h}时 ${m}分`;
		},

		formatBytes(bytes) {
			if (!bytes || bytes === 0) return '0 B';
			const units = ['B', 'KB', 'MB', 'GB', 'TB'];
			const i = Math.floor(Math.log(bytes) / Math.log(1024));
			return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
		},

		// 温度徽章：与总占用同行右侧展示。单热区只显示数值（处理器卡内
		// 无需再写“温度”），多热区（如 CPU + NPU）才带标签，用 · 分隔
		renderTempBadge(data) {
			const el = document.getElementById('cpu-temp');
			const zones = (data.thermals && data.thermals.length)
				? data.thermals
				: [{ type: 'cpu', temp: parseFloat(data.cpu_temp) || 0 }];
			el.innerHTML = zones.map(z => {
				const label = z.type.replace(/-thermal$/i, '').toUpperCase();
				const hot = z.temp > 60;
				const color = hot ? '#ef4444' : 'var(--text-main)';
				return `<span style="color:${color}">${zones.length > 1 ? label + ' ' : ''}${z.temp.toFixed(1)}°C</span>`;
			}).join('<span class="temp-sep"> · </span>');
		},

		// 每核占用：竖条迷你柱状图，柱下标签附带实时百分比
		renderCores(cores) {
			const grid = document.getElementById('core-grid');
			if (!cores || !cores.length) { grid.innerHTML = ''; return; }
			if (grid.childElementCount !== cores.length) {
				grid.innerHTML = cores.map((_, i) => `
					<div class="core-cell">
						<div class="core-bar-bg"><div class="core-bar" id="core-bar-${i}"></div></div>
						<span class="core-label" id="core-label-${i}">核${i}</span>
					</div>`).join('');
			}
			cores.forEach((v, i) => {
				const bar = document.getElementById(`core-bar-${i}`);
				if (bar) bar.style.height = Math.min(100, Math.max(0, v)) + '%';
				const label = document.getElementById(`core-label-${i}`);
				if (label) label.innerText = `核${i} ${Math.round(v)}%`;
			});
		},

		updateAll(data) {
			// 处理器（总占用 / 频率 / 温度区 / 每核 / 负载）
			document.getElementById('cpu-usage').innerText = data.cpu_usage.toFixed(1);
			document.getElementById('cpu-bar').style.width = data.cpu_usage + '%';
			document.getElementById('cpu-freq').innerText = Math.round(data.cpu_freq);
			this.renderTempBadge(data);
			this.renderCores(data.cpu_cores);

			// CPU 时间分解条（user/sys/iowait/idle 分段）
			const splitEl = document.getElementById('cpu-split');
			if (splitEl) {
				const segs = [
					{ v: data.cpu_user || 0, c: '#38bdf8' },
					{ v: data.cpu_sys || 0, c: '#818cf8' },
					{ v: data.cpu_iowait || 0, c: '#f87171' },
					{ v: data.cpu_idle || 0, c: 'rgba(255,255,255,0.15)' }
				];
				splitEl.innerHTML = segs.map(s =>
					`<span style="width:${Math.max(0, s.v)}%;background:${s.c};"></span>`).join('');
				document.getElementById('cpu-split-label').innerText =
					`user ${(data.cpu_user || 0).toFixed(0)}% · sys ${(data.cpu_sys || 0).toFixed(0)}% · iowait ${(data.cpu_iowait || 0).toFixed(0)}% · idle ${(data.cpu_idle || 0).toFixed(0)}%`;
			}

			document.getElementById('load-1').innerText = (data.load_1 || 0).toFixed(2);
			document.getElementById('load-5').innerText = (data.load_5 || 0).toFixed(2);
			document.getElementById('load-15').innerText = (data.load_15 || 0).toFixed(2);

			// 内存 / 交换
			document.getElementById('mem-usage').innerText = data.mem_usage.toFixed(1);
			document.getElementById('mem-bar').style.width = data.mem_usage + '%';
			document.getElementById('mem-summary').innerText = data.mem_summary;
			document.getElementById('mem-avail').innerText = this.formatBytes(data.mem_available);
			document.getElementById('mem-cached').innerText = this.formatBytes(data.mem_cached);
			document.getElementById('swap-usage').innerText = (data.swap_usage || 0).toFixed(1) + '%';
			document.getElementById('swap-bar').style.width = (data.swap_usage || 0) + '%';
			document.getElementById('swap-summary').innerText = data.swap_summary || '0 / 0 GB';

			// 网络（速率 + 累计 + 连接细分 + 丢包 + 外网连通）
			document.getElementById('net-down').innerText = data.net_down.toFixed(1);
			document.getElementById('net-up').innerText = data.net_up.toFixed(1);
			document.getElementById('net-total').innerText =
				`↓ ${this.formatBytes(data.net_total_down || 0)} · ↑ ${this.formatBytes(data.net_total_up || 0)}`;
			document.getElementById('net-conns').innerText =
				`${data.conn_established || 0} 已建立 · ${data.conn_listen || 0} 监听 · ${data.conn_timewait || 0} 等待`;
			document.getElementById('net-drops').innerText =
				`${data.net_dropped || 0} 丢包 · ${data.net_errors || 0} 错误`;
			const onlineEl = document.getElementById('net-online');
			onlineEl.innerHTML = data.net_online
				? `<span style="color:var(--success)">正常 · ${Math.round(data.net_latency_ms)} ms</span>`
				: '<span style="color:#ef4444">不可达</span>';

			// WiFi：驱动报真实 dBm 时优先用 dBm 分级，否则退回链路质量（满值 70）
			const wifiEl = document.getElementById('wifi-signal');
			if (data.wifi_dbm && data.wifi_dbm < 0) {
				const d = data.wifi_dbm;
				const grade = d >= -50 ? '优秀' : d >= -60 ? '良好' : d >= -70 ? '一般' : '较差';
				const color = d >= -60 ? 'var(--success)' : d >= -70 ? '#f59e0b' : '#ef4444';
				wifiEl.innerHTML = `<span style="color:${color}">${d.toFixed(0)} dBm · ${grade}</span>`;
			} else if (data.wifi_link && data.wifi_link > 0) {
				const q = Math.min(100, data.wifi_link / 70 * 100);
				const grade = q >= 85 ? '优秀' : q >= 70 ? '良好' : q >= 55 ? '一般' : '较差';
				const color = q >= 70 ? 'var(--success)' : q >= 55 ? '#f59e0b' : '#ef4444';
				wifiEl.innerHTML = `<span style="color:${color}">${data.wifi_link.toFixed(0)}/70 · ${grade}</span>`;
			} else {
				wifiEl.innerText = '未检测到';
			}

			// 磁盘（根占用大数值 + 其余挂载点行）
			document.getElementById('disk-usage').innerText = (data.disk_usage || 0).toFixed(1);
			document.getElementById('disk-bar').style.width = (data.disk_usage || 0) + '%';
			document.getElementById('disk-summary').innerText = data.disk_summary || '0 / 0 GB';
			document.getElementById('disk-read').innerText = (data.disk_read || 0).toFixed(1);
			document.getElementById('disk-write').innerText = (data.disk_write || 0).toFixed(1);
			const mountBox = document.getElementById('disk-mounts');
			if (mountBox) {
				mountBox.innerHTML = (data.disk_mounts || [])
					.filter(m => m.mount !== '/')
					.map(m => `<div class="info-row" style="margin-top:0.4rem;margin-bottom:0;"><span>${m.mount}</span><span>${(m.usage || 0).toFixed(1)}% · ${m.summary}</span></div>`)
					.join('');
			}
			const dq = document.getElementById('disk-quality');
			if (dq) {
				dq.innerText = `繁忙 ${(data.disk_busy || 0).toFixed(0)}% · 延迟 ${(data.disk_latency_ms || 0).toFixed(1)}ms · ${Math.round(data.disk_iops || 0)} IOPS`;
			}

			// 进程 TOP5（名称来自 /proc，做基本转义防注入）
			const procBox = document.getElementById('proc-rows');
			if (procBox) {
				procBox.innerHTML = (data.top_procs || []).map(p => {
					const name = String(p.name).replace(/[<>&]/g, ch => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[ch]));
					return `<div class="proc-row"><span class="proc-name">${name}<span class="proc-pid">#${p.pid}</span></span><span class="proc-val">${(p.cpu || 0).toFixed(1)}% · ${(p.mem || 0).toFixed(1)}%</span></div>`;
				}).join('') || '<p class="card-subtitle">--</p>';
			}

			// 页头：运行时间 + 状态 + 最后更新
			document.getElementById('local-time').innerText =
				`已运行 ${this.formatUptime(data.uptime || 0)} | 系统状态正常 | 最后更新: ${new Date().toLocaleTimeString()}`;
		}
	};

	// ===== 24 小时趋势图（数据来自 /api/history，每 10 秒一个点） =====
	const Trends = {
		data: null,
		metric: 'cpu',
		range: 24,
		METRICS: {
			cpu: [{ key: 'cpu', color: '#38bdf8', label: 'CPU %' }],
			temp: [{ key: 'temp', color: '#f87171', label: '温度 °C' }],
			mem: [{ key: 'mem', color: '#34d399', label: '内存 %' }],
			net: [
				{ key: 'net_down', color: '#38bdf8', label: '下载 KB/s' },
				{ key: 'net_up', color: '#c084fc', label: '上传 KB/s' }
			]
		},

		setData(data) {
			this.data = data;
			this.render();
		},

		fmtVal(v) {
			return v >= 100 ? Math.round(v).toString() : v.toFixed(1);
		},

		render() {
			const canvas = document.getElementById('trend-canvas');
			if (!canvas) return;
			const series = this.METRICS[this.metric];

			document.getElementById('chart-legend').innerHTML = series.map(s =>
				`<span class="legend-item"><span class="legend-dot" style="background:${s.color}"></span>${s.label}</span>`
			).join('');

			const ctx = canvas.getContext('2d');
			const dpr = window.devicePixelRatio || 1;
			const cssW = canvas.clientWidth || 600;
			const cssH = 220;
			canvas.width = cssW * dpr;
			canvas.height = cssH * dpr;
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			ctx.clearRect(0, 0, cssW, cssH);

			if (!this.data || !this.data.t || this.data.t.length < 2) {
				ctx.fillStyle = '#94a3b8';
				ctx.font = '13px system-ui';
				ctx.textAlign = 'center';
				ctx.fillText('数据积累中...', cssW / 2, cssH / 2);
				return;
			}

			// 截取所选时间范围
			const maxPoints = Math.floor(this.range * 3600 / this.data.interval_s);
			const start = Math.max(0, this.data.t.length - maxPoints);
			const t = this.data.t.slice(start);
			const cols = {};
			for (const s of series) cols[s.key] = this.data[s.key].slice(start);
			const n = t.length;

			// 跨系列求 Y 轴范围，留边距并以 0 为下限
			let lo = Infinity, hi = -Infinity;
			for (const s of series) {
				for (const v of cols[s.key]) {
					if (v < lo) lo = v;
					if (v > hi) hi = v;
				}
			}
			if (!isFinite(lo)) { lo = 0; hi = 1; }
			if (hi - lo < 1e-6) hi = lo + 1;
			const pad = (hi - lo) * 0.1;
			lo = Math.max(0, lo - pad);
			hi += pad;

			const M = { l: 46, r: 8, t: 6, b: 18 };
			const W = cssW - M.l - M.r;
			const H = cssH - M.t - M.b;
			const px = i => M.l + (i / (n - 1)) * W;
			const py = v => M.t + (1 - (v - lo) / (hi - lo)) * H;

			// 网格线 + Y 轴刻度
			ctx.font = '11px system-ui';
			for (let g = 0; g <= 4; g++) {
				const gy = M.t + (g / 4) * H;
				ctx.strokeStyle = 'rgba(255,255,255,0.08)';
				ctx.beginPath();
				ctx.moveTo(M.l, gy);
				ctx.lineTo(M.l + W, gy);
				ctx.stroke();
				ctx.fillStyle = '#64748b';
				ctx.textAlign = 'right';
				ctx.fillText(this.fmtVal(hi - (g / 4) * (hi - lo)), M.l - 6, gy + 3);
			}

			// X 轴时间标签：首 / 中 / 尾
			ctx.fillStyle = '#64748b';
			[[0, 'left'], [0.5, 'center'], [1, 'right']].forEach(([frac, align]) => {
				const idx = Math.round(frac * (n - 1));
				const label = new Date(t[idx] * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
				ctx.textAlign = align;
				ctx.fillText(label, M.l + frac * W, cssH - 4);
			});

			// 绘制各序列折线
			for (const s of series) {
				ctx.strokeStyle = s.color;
				ctx.lineWidth = 1.5;
				ctx.beginPath();
				for (let i = 0; i < n; i++) {
					if (i) ctx.lineTo(px(i), py(cols[s.key][i]));
					else ctx.moveTo(px(i), py(cols[s.key][i]));
				}
				ctx.stroke();
			}
		},

		wireControls() {
			const activate = (btn, group) => {
				group.forEach(b => b.classList.toggle('active', b === btn));
			};
			const metricBtns = document.querySelectorAll('.chart-metrics .chart-btn');
			metricBtns.forEach(btn => btn.addEventListener('click', () => {
				this.metric = btn.dataset.metric;
				activate(btn, metricBtns);
				this.render();
			}));
			const rangeBtns = document.querySelectorAll('.chart-ranges .chart-btn');
			rangeBtns.forEach(btn => btn.addEventListener('click', () => {
				this.range = parseInt(btn.dataset.range, 10);
				activate(btn, rangeBtns);
				this.render();
			}));
			window.addEventListener('resize', () => this.render());
		}
	};

	let failCount = 0;
	// 轮询间隔与后端快档采样对齐（2 秒），避免一半请求拿到重复数据
	let delay = 2000;

	function scheduleNext() {
		setTimeout(fetchStats, delay);
	}

	async function fetchStats() {
		try {
			const response = await fetch('/api/stats');
			if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

			const data = await response.json();
			UI.updateAll(data);
			failCount = 0;
			delay = 2000;
		} catch (error) {
			failCount++;
			delay = Math.min(delay * 2, 30000);
			document.getElementById('local-time').innerText = `连接后端失败 (重试次数: ${failCount})...`;
			console.error("Failed to fetch stats:", error);
		}
		scheduleNext();
	}

	async function fetchHistory() {
		try {
			const response = await fetch('/api/history');
			if (!response.ok) return;
			Trends.setData(await response.json());
		} catch (error) {
			console.error("Failed to fetch history:", error);
		}
	}

	// ===== 系统信息（静态，仅加载时拉取一次；处理器行随轮询带实时频率） =====
	let Sys = null;
	async function fetchSystem() {
		try {
			const response = await fetch('/api/system');
			if (!response.ok) return;
			const s = await response.json();
			Sys = s;
			// 页面标题跟随主板型号，多平台自动正确（OrangePi / RK 等）
			const pageTitle = s.board ? `${s.board} 系统监控` : '系统监控';
			document.getElementById('page-title').innerText = pageTitle;
			document.title = pageTitle;
			document.getElementById('sys-os').innerText = s.os || '--';
			document.getElementById('sys-kernel').innerText = s.kernel || '--';
			document.getElementById('sys-board').innerText = s.board || '--';
			document.getElementById('sys-version').innerText = s.version || '--';
			// 网页终端：后端开启（MONITOR_TERMINAL=1）才显示入口按钮
			const tBtn = document.getElementById('terminal-toggle');
			if (tBtn && s.terminal) {
				tBtn.style.display = '';
				tBtn.addEventListener('click', () => window.TerminalPanel && TerminalPanel.toggle());
			}
			// 主频是规格参数（静态）；实时频率在处理器卡里随轮询更新
			document.getElementById('sys-cpu').innerText = s.cpu_max_ghz > 0
				? `${s.cpu} @ ${s.cpu_max_ghz.toFixed(2)} GHz`
				: (s.cpu || '--');
		} catch (error) {
			console.error('Failed to fetch system info:', error);
		}
	}

	document.addEventListener('DOMContentLoaded', () => {
		fetchStats();
		fetchSystem();
		Trends.wireControls();
		fetchHistory();
		setInterval(fetchHistory, 60000);
	});
})();
