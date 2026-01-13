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
		document.getElementById('local-time').innerText =
			`系统状态正常 | 最后更新: ${new Date().toLocaleTimeString()}`;
	}
};

// ==========================================
// 自动化寻址配置
// ==========================================
// 请将下方的链接替换为你 Gist 的 Raw 按钮点击后的真实链接
const GIST_RAW_URL = "https://gist.githubusercontent.com/632-8nm/39872bc42a8a45a854c982f8016185bd/raw/orangepi_url.json";

let cachedApiBase = null;

/**
 * 第一步：从 Gist 获取最新的隧道域名
 */
async function getLiveApiBase() {
	try {
		// 加上时间戳缓存破坏符，确保拿到的不是浏览器旧缓存
		const response = await fetch(`${GIST_RAW_URL}?t=${Date.now()}`, { cache: "no-store" });
		if (!response.ok) throw new Error('无法读取 Gist 配置');
		const config = await response.json();
		return config.url; // 返回如 https://xxx.trycloudflare.com
	} catch (error) {
		console.error('寻址失败:', error);
		return null;
	}
}

/**
 * 第二步：抓取监控数据
 */
async function fetchStats() {
	try {
		// 1. 如果还没有域名，先获取域名
		if (!cachedApiBase) {
			cachedApiBase = await getLiveApiBase();
		}

		if (!cachedApiBase) {
			document.getElementById('local-time').innerText = "正在寻找后端入口...";
			return;
		}

		// 2. 使用拿到的域名请求数据
		const response = await fetch(`${cachedApiBase}/api/stats`);
		if (!response.ok) {
			// 如果请求失败，可能是域名失效了，清除缓存下次尝试重新寻址
			cachedApiBase = null;
			throw new Error('后端连接失效');
		}

		const data = await response.json();

		// 3. 更新 UI
		UI.updateCPU(data.cpu_usage);
		UI.updateTemp(data.cpu_temp);
		UI.updateMemory(data.mem_usage, data.mem_summary);
		UI.updateTime();

	} catch (error) {
		console.error('获取数据失败:', error);
		document.getElementById('local-time').innerText = "连接中断，正在尝试重连...";
	}
}

// 初始化
document.addEventListener('DOMContentLoaded', () => {
	// 立即执行一次
	fetchStats();
	// 每 1000 毫秒刷新一次
	setInterval(fetchStats, 1000);
});