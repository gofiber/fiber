package monitor

import (
	"strconv"
	"strings"
	"time"
)

type viewBag struct {
	title      string
	refresh    time.Duration
	fontURL    string
	chartJSURL string
	customHead string
}

// returns index with new title/refresh
func newIndex(dat viewBag) string {
	timeout := dat.refresh.Milliseconds() - timeoutDiff
	if timeout < timeoutDiff {
		timeout = timeoutDiff
	}
	ts := strconv.FormatInt(timeout, 10)
	replacer := strings.NewReplacer("$TITLE", dat.title, "$TIMEOUT", ts,
		"$FONT_URL", dat.fontURL, "$CHART_JS_URL", dat.chartJSURL, "$CUSTOM_HEAD", dat.customHead,
	)
	return replacer.Replace(indexHTML)
}

const (
	defaultTitle = "Fiber Monitor"

	defaultRefresh    = 3 * time.Second
	timeoutDiff       = 200 // timeout will be Refresh (in milliseconds) - timeoutDiff
	minRefresh        = timeoutDiff * time.Millisecond
	defaultFontURL    = `https://fonts.googleapis.com/css2?family=Roboto:wght@400;900&display=swap`
	defaultChartJSURL = `https://cdn.jsdelivr.net/npm/chart.js@2.9/dist/Chart.bundle.min.js`
	defaultCustomHead = ``

	// parametrized by $TITLE and $TIMEOUT
	indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link href="$FONT_URL" rel="stylesheet">
	<script src="$CHART_JS_URL"></script>

	<title>$TITLE</title>
<style>
	body {
		margin: 0;
		font: 16px / 1.6 'Roboto', sans-serif;
	}
	.wrapper {
		max-width: 900px;
		margin: 0 auto;
		padding: 30px 0;
	}
	.title {
		text-align: center;
		margin-bottom: 2em;
	}
	.title h1 {
		font-size: 1.8em;
		padding: 0;
		margin: 0;
	}
	.row {
		display: flex;
		margin-bottom: 20px;
		align-items: center;
	}
	.row .column:first-child { width: 35%; }
	.row .column:last-child { width: 65%; }
	.metric {
		color: #777;
		font-weight: 900;
	}
	h2 {
		padding: 0;
		margin: 0;
		font-size: 2.2em;
	}
	h2 span {
		font-size: 12px;
		color: #777;
	}
	h2 span.ram_os { color: rgba(255, 150, 0, .8); }
	h2 span.ram_total { color: rgba(0, 200, 0, .8); }
	canvas {
		width: 200px;
		height: 180px;
	}
$CUSTOM_HEAD
</style>
</head>
<body>
	<section class="wrapper">
	<div class="title"><h1>$TITLE</h1></div>
	<section class="charts">
		<div class="row">
			<div class="column">
				<div class="metric">CPU Usage</div>
				<h2 id="cpuMetric">0.00%</h2>
			</div>
			<div class="column">
				<canvas id="cpuChart"></canvas>
			</div>
		</div>
		<div class="row">
			<div class="column">
				<div class="metric">Memory Usage</div>
				<h2 id="ramMetric" title="PID used / OS used / OS total">0.00 MB</h2>
			</div>
			<div class="column">
				<canvas id="ramChart"></canvas>
			</div>
		</div>
		<div class="row">
			<div class="column">
				<div class="metric">Response Time</div>
				<h2 id="rtimeMetric">0ms</h2>
			</div>
			<div class="column">
				<canvas id="rtimeChart"></canvas>
			</div>
		</div>
		<div class="row">
			<div class="column">
				<div class="metric">Open Connections</div>
				<h2 id="connsMetric">0</h2>
			</div>
			<div class="column">
				<canvas id="connsChart"></canvas>
			</div>
		</div>
	</section>
	</section>
<script>
	function formatBytes(bytes, decimals = 1) {
		if (bytes === 0) return '0 Bytes';

		const k = 1024;
		const dm = decimals < 0 ? 0 : decimals;
		const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

		const i = Math.floor(Math.log(bytes) / Math.log(k));

		return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
	}
	Chart.defaults.global.legend.display = false;
	Chart.defaults.global.defaultFontSize = 8;
	Chart.defaults.global.animation.duration = 1000;
	Chart.defaults.global.animation.easing = 'easeOutQuart';
	Chart.defaults.global.elements.line.backgroundColor = 'rgba(0, 172, 215, 0.25)';
	Chart.defaults.global.elements.line.borderColor = 'rgba(0, 172, 215, 1)';
	Chart.defaults.global.elements.line.borderWidth = 2;

	const options = {
		scales: {
			yAxes: [{ ticks: { beginAtZero: true }}],
			xAxes: [{
				type: 'time',
				time: {
					unitStepSize: 30,
					unit: 'second'
				},
				gridlines: { display: false }
			}]
		},
		tooltips: {	enabled: false },
		responsive: true,
		maintainAspectRatio: false,
		animation: false
	};
	const cpuMetric = document.querySelector('#cpuMetric');
	const ramMetric = document.querySelector('#ramMetric');
	const rtimeMetric = document.querySelector('#rtimeMetric');
	const connsMetric = document.querySelector('#connsMetric');

	const cpuChartCtx = document.querySelector('#cpuChart').getContext('2d');
	const ramChartCtx = document.querySelector('#ramChart').getContext('2d');
	const rtimeChartCtx = document.querySelector('#rtimeChart').getContext('2d');
	const connsChartCtx = document.querySelector('#connsChart').getContext('2d');

	const cpuChart = createChart(cpuChartCtx);
	const ramChart = createChart(ramChartCtx);
	const rtimeChart = createChart(rtimeChartCtx);
	const connsChart = createChart(connsChartCtx);

	const charts = [cpuChart, ramChart, rtimeChart, connsChart];

	function createChart(ctx) {
		return new Chart(ctx, {
			type: 'line',
			data: {
				labels: [],
				datasets: [{
					label: '',
					data: [],
					lineTension: 0.2,
					pointRadius: 0,
				}]
			},
			options
		});
	}
	ramChart.data.datasets.push({
		data: [],
		lineTension: 0.2,
		pointRadius: 0,
		backgroundColor: 'rgba(255, 200, 0, .6)',
		borderColor: 'rgba(255, 150, 0, .8)',
	})
	ramChart.data.datasets.push({
		data: [],
		lineTension: 0.2,
		pointRadius: 0,
		backgroundColor: 'rgba(0, 255, 0, .4)',
		borderColor: 'rgba(0, 200, 0, .8)',
	})
	function update(json, rtime) {
		cpu = json.pid.cpu.toFixed(1);
		cpuOS = json.os.cpu.toFixed(1);

		cpuMetric.innerHTML = cpu + '% <span>' + cpuOS + '%</span>';
		ramMetric.innerHTML = formatBytes(json.pid.ram) + '<span> / </span><span class="ram_os">' + formatBytes(json.os.ram) +
			'<span><span> / </span><span class="ram_total">' + formatBytes(json.os.total_ram) + '</span>';
		rtimeMetric.innerHTML = rtime + 'ms <span>client</span>';
		connsMetric.innerHTML = json.pid.conns + ' <span>' + json.os.conns + '</span>';

		cpuChart.data.datasets[0].data.push(cpu);
		ramChart.data.datasets[2].data.push((json.os.total_ram / 1e6).toFixed(2));
		ramChart.data.datasets[1].data.push((json.os.ram / 1e6).toFixed(2));
		ramChart.data.datasets[0].data.push((json.pid.ram / 1e6).toFixed(2));
		rtimeChart.data.datasets[0].data.push(rtime);
		connsChart.data.datasets[0].data.push(json.pid.conns);

		const timestamp = new Date().getTime();

		charts.forEach(chart => {
			if (chart.data.labels.length > 50) {
				chart.data.datasets.forEach(function (dataset) { dataset.data.shift(); });
				chart.data.labels.shift();
			}
			chart.data.labels.push(timestamp);
			chart.update();
		});
		setTimeout(fetchJSON, $TIMEOUT)
	}
	function fetchJSON() {
		var t1 = ''
		var t0 = performance.now()
		fetch(window.location.href, {
				headers: { 'Accept': 'application/json' },
				credentials: 'same-origin'
			})
			.then(res => {
				t1 = performance.now()
				return res.json()
			})
			.then(res => { update(res, Math.round(t1 - t0)) })
			.catch(console.error);
	}
	fetchJSON()
</script>
</body>
</html>
`
)
