const UI = {
	// 格式化运行时间
	formatUptime(seconds) {
		const d = Math.floor(seconds / (3600 * 24));
		const h = Math.floor((seconds % (3600 * 24)) / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		return `${d}天 ${h}时 ${m}分`;
	},

	updateAll(data) {
		// CPU
		document.getElementById('cpu-usage').innerText = data.cpu_usage.toFixed(1);
		document.getElementById('cpu-bar').style.width = data.cpu_usage + '%';
		document.getElementById('cpu-freq').innerText = Math.round(data.cpu_freq);
		const tempEl = document.getElementById('cpu-temp');
		tempEl.innerText = data.cpu_temp;
		tempEl.style.color = parseFloat(data.cpu_temp) > 60 ? '#ef4444' : '#f8fafc';

		// Memory
		document.getElementById('mem-usage').innerText = data.mem_usage.toFixed(1);
		document.getElementById('mem-bar').style.width = data.mem_usage + '%';
		document.getElementById('mem-summary').innerText = data.mem_summary;

		// System
		document.getElementById('sys-os').innerText = data.os_info;
		document.getElementById('sys-uptime').innerText = this.formatUptime(data.uptime);

		// Network
		document.getElementById('net-down').innerText = data.net_down.toFixed(1);
		document.getElementById('net-up').innerText = data.net_up.toFixed(1);

		document.getElementById('local-time').innerText =
			`系统状态正常 | 最后更新: ${new Date().toLocaleTimeString()}`;
	}
};

const GIST_RAW_URL = "https://gist.githubusercontent.com/632-8nm/39872bc42a8a45a854c982f8016185bd/raw/orangepi_url.json";
let cachedApiBase = null;
let failCount = 0;

async function getLiveApiBase() {
	try {
		const response = await fetch(`${GIST_RAW_URL}?t=${Date.now()}`, { cache: "no-store" });
		const config = await response.json();
		return config.url;
	} catch (e) { return null; }
}

async function fetchStats() {
	try {
		if (!cachedApiBase) cachedApiBase = await getLiveApiBase();
		if (!cachedApiBase) return;

		const response = await fetch(`${cachedApiBase}/api/stats`);
		if (!response.ok) throw new Error();

		const data = await response.json();
		UI.updateAll(data);
		failCount = 0;
	} catch (error) {
		failCount++;
		if (failCount >= 3) {
			cachedApiBase = null;
			document.getElementById('local-time').innerText = "正在重新寻址后端...";
		}
	}
}

document.addEventListener('DOMContentLoaded', () => {
	fetchStats();
	setInterval(fetchStats, 1000);
});