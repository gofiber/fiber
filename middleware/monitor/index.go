package monitor

var index = []byte(`<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link href="https://fonts.googleapis.com/css2?family=Roboto:wght@400;900&display=swap" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/chart.js@2.8.0/dist/Chart.bundle.min.js"></script>
    <title>Fiber Status Monitor</title>
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

    .row .column:first-child {
        width: 35%;
    }

    .row .column:last-child {
        width: 65%;
    }

    .metric {
        color: #777;
        font-weight: 900;
    }

    h2 {
        padding: 0;
        margin: 0;
        font-size: 2.2em;
    }

    canvas {
        width: 200px;
        height: 180px;
    }
    </style>
</head>

<body>
    <section class="wrapper">
        <div class="title">
            <h1>Fiber Status Monitor</h1>
        </div>

        <section class="charts">
            <div class="row">
                <div class="column">
                    <div class="metric">CPU:</div>
                    <h2 id="cpuMetric">0.00%</h2>
                </div>
                <div class="column">
                    <canvas id="cpuChart"></canvas>
                </div>
            </div>

            <div class="row">
                <div class="column">
                    <div class="metric">RAM:</div>
                    <h2 id="ramMetric">0.00 MB</h2>
                </div>
                <div class="column">
                    <canvas id="ramChart"></canvas>
                </div>
            </div>

            <div class="row">
                <div class="column">
                    <div class="metric">Load:</div>
                    <h2 id="loadMetric">0.00%</h2>
                </div>
                <div class="column">
                    <canvas id="loadChart"></canvas>
                </div>
            </div>

            <div class="row">
                <div class="column">
                    <div class="metric">Latest Response Time:</div>
                    <h2 id="timeMetric">0ms</h2>
                </div>
                <div class="column">
                    <canvas id="timeChart"></canvas>
                </div>
            </div>

            <div class="row">
                <div class="column">
                    <div class="metric">Requests:</div>
                    <h2 id="reqMetric">0</h2>
                </div>
                <div class="column">
                    <canvas id="reqChart"></canvas>
                </div>
            </div>
        </section>
    </section>

    <script>
    Chart.defaults.global.legend.display = false;
    Chart.defaults.global.defaultFontSize = 8;
    Chart.defaults.global.animation.duration = 500;
    Chart.defaults.global.elements.line.backgroundColor = 'rgba(0, 0, 0, 0)';
    Chart.defaults.global.elements.line.borderColor = 'rgba(0, 0, 0, 0.9)';
    Chart.defaults.global.elements.line.borderWidth = 2;

    const options = {
        scales: {
            yAxes: [
                {
                    ticks: {
                        beginAtZero: false
                    }
                }
            ],
            xAxes: [
                {
                    type: 'time',
                    time: {
                        unitStepSize: 30,
                        unit: 'second'
                    },
                    gridlines: {
                        display: false
                    }
                }
            ]
        },
        tooltips: {
            enabled: false
        },
        responsive: true,
        maintainAspectRatio: false,
        animation: false
    };

    const cpuMetric = document.querySelector('#cpuMetric');
    const ramMetric = document.querySelector('#ramMetric');
    const loadMetric = document.querySelector('#loadMetric');
    const timeMetric = document.querySelector('#timeMetric');
    const reqMetric = document.querySelector('#reqMetric');

    const cpuChartCtx = document.querySelector('#cpuChart').getContext('2d');
    const ramChartCtx = document.querySelector('#ramChart').getContext('2d');
    const loadChartCtx = document.querySelector('#loadChart').getContext('2d');
    const timeChartCtx = document.querySelector('#timeChart').getContext('2d');
    const reqChartCtx = document.querySelector('#reqChart').getContext('2d');

    const cpuChart = createChart(cpuChartCtx);
    const ramChart = createChart(ramChartCtx);
    const loadChart = createChart(loadChartCtx);
    const timeChart = createChart(timeChartCtx);
    const reqChart = createChart(reqChartCtx);

    const charts = [ cpuChart, ramChart, loadChart, timeChart, reqChart ];

    function createChart(ctx) {
        return new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: '',
                        data: [],
                        lineTension: 0.2,
                        pointRadius: 0
                    }
                ]
            },
            options
        });
    }

    function update({ cpu, ram, load, time, reqs }) {
        cpu = cpu.toFixed(2);
        ram = (ram / 1e6).toFixed(2);
        load = load.toFixed(2);

        cpuMetric.innerHTML = cpu + '%';
        ramMetric.innerHTML = ram + ' MB';
        loadMetric.innerHTML = load;
        timeMetric.innerHTML = time + 'ms';
        reqMetric.innerHTML = reqs;

        cpuChart.data.datasets[0].data.push(cpu);
        ramChart.data.datasets[0].data.push(ram);
        loadChart.data.datasets[0].data.push(load);
        timeChart.data.datasets[0].data.push(time);
        reqChart.data.datasets[0].data.push(reqs);

        const timestamp = new Date().getTime();

        charts.forEach(chart => {
            chart.data.labels.push(timestamp);
            chart.update();
        });
    }

    setInterval(function () {
        fetch('/status', {
            headers: {
                'Accept': 'application/json'
            },
            credentials: 'same-origin'
        })
            .then(res => res.json())
            .then(update)
            .catch(console.error);
    }, 1000);
    </script>
</body>

</html>
`)
