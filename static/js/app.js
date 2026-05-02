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

		updateAll(data) {
			// CPU
			document.getElementById('cpu-usage').innerText = data.cpu_usage.toFixed(1);
			document.getElementById('cpu-bar').style.width = data.cpu_usage + '%';
			document.getElementById('cpu-freq').innerText = Math.round(data.cpu_freq);
			const tempEl = document.getElementById('cpu-temp');
			tempEl.innerText = data.cpu_temp;
			tempEl.style.color = parseFloat(data.cpu_temp) > 60 ? '#ef4444' : '#f8fafc';

			// Load
			document.getElementById('load-1').innerText = (data.load_1 || 0).toFixed(2);
			document.getElementById('load-5').innerText = (data.load_5 || 0).toFixed(2);
			document.getElementById('load-15').innerText = (data.load_15 || 0).toFixed(2);

			// Memory
			document.getElementById('mem-usage').innerText = data.mem_usage.toFixed(1);
			document.getElementById('mem-bar').style.width = data.mem_usage + '%';
			document.getElementById('mem-summary').innerText = data.mem_summary;
			document.getElementById('mem-avail').innerText = this.formatBytes(data.mem_available);
			document.getElementById('mem-cached').innerText = this.formatBytes(data.mem_cached);

			// Swap
			document.getElementById('swap-usage').innerText = (data.swap_usage || 0).toFixed(1);
			document.getElementById('swap-bar').style.width = (data.swap_usage || 0) + '%';
			document.getElementById('swap-summary').innerText = data.swap_summary || '0 / 0 GB';

			// Network
			document.getElementById('net-down').innerText = data.net_down.toFixed(1);
			document.getElementById('net-up').innerText = data.net_up.toFixed(1);
			document.getElementById('net-conns').innerText = data.connections || 0;

			// Disk
			document.getElementById('disk-usage').innerText = (data.disk_usage || 0).toFixed(1);
			document.getElementById('disk-bar').style.width = (data.disk_usage || 0) + '%';
			document.getElementById('disk-summary').innerText = data.disk_summary || '0 / 0 GB';
			document.getElementById('disk-read').innerText = (data.disk_read || 0).toFixed(1);
			document.getElementById('disk-write').innerText = (data.disk_write || 0).toFixed(1);

			document.getElementById('local-time').innerText =
				`系统状态正常 | 最后更新: ${new Date().toLocaleTimeString()}`;
		}
	};

	let failCount = 0;
	let delay = 1000;

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
			delay = 1000;
		} catch (error) {
			failCount++;
			delay = Math.min(delay * 2, 30000);
			document.getElementById('local-time').innerText = `连接后端失败 (重试次数: ${failCount})...`;
			console.error("Failed to fetch stats:", error);
		}
		scheduleNext();
	}

	document.addEventListener('DOMContentLoaded', () => {
		fetchStats();
	});
})();
