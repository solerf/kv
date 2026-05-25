document.addEventListener("DOMContentLoaded", function () {
    const ctx = document.getElementById("pod-context").value;
    const ns = document.getElementById("pod-namespace").value;
    const name = document.getElementById("pod-name").value;
    const params =
        "context=" +
        encodeURIComponent(ctx) +
        "&namespace=" +
        encodeURIComponent(ns) +
        "&name=" +
        encodeURIComponent(name);

    // Describe
    const describeOutput = document.getElementById("describe-output");
    fetch("/api/describe?" + params + "&kind=pods")
        .then((r) => r.text())
        .then((yaml) => {
            describeOutput.textContent = yaml;
        })
        .catch((err) => {
            describeOutput.textContent = "Error: " + err.message;
        });

    const cpuData = {
        labels: [],
        datasets: [{
            label: "CPU (m)",
            data: [],
            borderColor: "#000",
            backgroundColor: "rgba(0,0,0,0.1)",
            tension: 0.3,
            fill: true
        }]
    };
    const memData = {
        labels: [],
        datasets: [{
            label: "Memory (Mi)",
            data: [],
            borderColor: "#ff4136",
            backgroundColor: "rgba(255,65,54,0.1)",
            tension: 0.3,
            fill: true
        }]
    };

    const cpuChartOpts = {
        responsive: true,
        animation: false,
        scales: {
            x: {display: true},
            y: {beginAtZero: true},
        },
        plugins: {legend: {display: false}},
    };

    const memChartOpts = {
        responsive: true,
        animation: false,
        scales: {
            x: {display: true},
            y: {
                beginAtZero: true,
                ticks: {
                    callback: function (value) {
                        return value + " MB";
                    },
                },
            },
        },
        plugins: {legend: {display: false}},
    };

    const cpuChart = new Chart(document.getElementById("cpu-chart"), {
        type: "line",
        data: cpuData,
        options: cpuChartOpts,
    });

    const memChart = new Chart(document.getElementById("mem-chart"), {
        type: "line",
        data: memData,
        options: memChartOpts,
    });

    function pollMetrics() {
        fetch("/api/pod/metrics?" + params)
            .then((r) => {
                if (!r.ok) throw new Error("metrics unavailable");
                return r.json();
            })
            .then((m) => {
                const now = new Date().toLocaleTimeString();
                cpuData.labels.push(now);
                cpuData.datasets[0].data.push(m.cpu);
                memData.labels.push(now);
                memData.datasets[0].data.push(m.memory);

                if (cpuData.labels.length > 30) {
                    cpuData.labels.shift();
                    cpuData.datasets[0].data.shift();
                    memData.labels.shift();
                    memData.datasets[0].data.shift();
                }

                cpuChart.update();
                memChart.update();
            })
            .catch(() => {
            });
    }

    pollMetrics();
    setInterval(pollMetrics, 5000);

    // Logs
    const logsModal = new bootstrap.Modal(document.getElementById("logs-modal"));
    const logsOutput = document.getElementById("logs-output");
    let logsController = null;

    document.getElementById("btn-logs").addEventListener("click", function () {
        logsOutput.textContent = "Loading...";
        logsModal.show();

        if (logsController) logsController.abort();
        logsController = new AbortController();

        fetch("/api/pod/logs?" + params + "&follow=true", {
            signal: logsController.signal,
        })
            .then((r) => {
                const reader = r.body.getReader();
                const decoder = new TextDecoder();
                logsOutput.textContent = "";

                function read() {
                    reader
                        .read()
                        .then(({done, value}) => {
                            if (done) return;
                            logsOutput.textContent += decoder.decode(value);
                            logsOutput.scrollTop = logsOutput.scrollHeight;
                            read();
                        })
                        .catch(() => {
                        });
                }

                read();
            })
            .catch(() => {
            });
    });

    document.getElementById("logs-modal").addEventListener("hidden.bs.modal", function () {
        if (logsController) {
            logsController.abort();
            logsController = null;
        }
    });

    // Delete
    const btnDelete = document.getElementById("btn-delete");
    const enableDelete = document.getElementById("enable-delete");

    enableDelete.addEventListener("change", function () {
        btnDelete.disabled = !this.checked;
    });

    btnDelete.addEventListener("click", function () {
        if (!confirm("Delete pod " + name + "?")) return;

        fetch("/api/pod?" + params, {method: "DELETE"})
            .then((r) => {
                if (!r.ok) throw new Error("delete failed");
                return r.json();
            })
            .then(() => {
                alert("Pod deleted");
                window.location.href =
                    "/?context=" + encodeURIComponent(ctx) + "&namespace=" + encodeURIComponent(ns);
            })
            .catch((err) => {
                alert("Error: " + err.message);
            });
    });
});
