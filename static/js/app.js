/**
 * UI 更新逻辑封装
 */
const UI = {
	updateCPU(usage) {
		document.getElementById('cpu-usage').innerText = usage.toFixed(1);
		document.getElementById('cpu-bar').style.width = usage + '%';
	},

	updateTemp(tempStr) {
		const tempEl = document.getElementById('cpu-temp');
		tempEl.innerText = tempStr;
		const tempVal = parseFloat(tempStr);
		tempEl.style.color = tempVal > 60 ? 'var(--danger)' : 'var(--text-main)';
	},

	updateMemory(usage, summary) {
		document.getElementById('mem-usage').innerText = usage.toFixed(1);
		document.getElementById('mem-bar').style.width = usage + '%';
		document.getElementById('mem-summary').innerText = summary;
	},

	updateTime() {
		// 修改为中文显示
		document.getElementById('local-time').innerText =
			`系统状态正常 | 最后更新: ${new Date().toLocaleTimeString()}`;
	}
};

async function fetchStats() {
	try {
		// const response = await fetch('/api/stats');
		// const response = await fetch('http://192.168.124.21:8080/api/stats');
		const response = await fetch('https://bat-glossary-garmin-studying.trycloudflare.com/api/stats');
		if (!response.ok) throw new Error('网络异常');

		const data = await response.json();

		UI.updateCPU(data.cpu_usage);
		UI.updateTemp(data.cpu_temp);
		UI.updateMemory(data.mem_usage, data.mem_summary);
		UI.updateTime();

	} catch (error) {
		console.error('获取数据失败:', error);
		document.getElementById('local-time').innerText = "连接中断，正在尝试重连...";
	}
}

document.addEventListener('DOMContentLoaded', () => {
	fetchStats();
	setInterval(fetchStats, 1000);
});