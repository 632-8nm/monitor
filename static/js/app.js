const UI = {
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

let failCount = 0;

async function fetchStats() {
	try {
		const response = await fetch('/api/stats');
		if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

		const data = await response.json();
		UI.updateAll(data);
		failCount = 0; // 请求成功，重置失败计数
	} catch (error) {
		failCount++;
		document.getElementById('local-time').innerText = `连接后端失败 (重试次数: ${failCount})...`;
		console.error("Failed to fetch stats:", error);
	}
}

document.addEventListener('DOMContentLoaded', () => {
	fetchStats();
	setInterval(fetchStats, 1000);
});