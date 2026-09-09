document.addEventListener("DOMContentLoaded", function () {
    const ctxSelect = document.getElementById("ctx-select");
    const nsSelect = document.getElementById("ns-select");
    const resourcesSection = document.getElementById("resources-section");
    const podsSection = document.getElementById("pods-section");
    const describeModal = new bootstrap.Modal(document.getElementById("describe-modal"));
    const describeTitle = document.getElementById("describe-modal-title");
    const describeBody = document.getElementById("describe-modal-body");

    const podColumns = ["", "Name", "Status", "Image", "Age", "Labels"];

    // Escapes for both text and attribute contexts (quotes included).
    function escapeHTML(value) {
        return (value == null ? "" : String(value))
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#39;");
    }

    // On an error status the backend sends the message as plain text.
    function jsonOrThrow(r) {
        if (r.ok) return r.json();
        return r.text().then((msg) => {
            throw new Error(msg.trim() || r.statusText);
        });
    }

    function statusClass(status) {
        if (status === "Running" || status === "Succeeded") return "status-running";
        if (status === "Pending" || status === "ContainerCreating") return "status-pending";
        if (status === "Failed" || /BackOff|Error|Crash|Evicted|Invalid/i.test(status)) return "status-failed";
        return "";
    }

    // Sorted key=value badges; empty string when there are no labels.
    function labelBadges(labels) {
        const keys = Object.keys(labels || {}).sort();
        if (keys.length === 0) return "";
        let html = '<div class="label-list">';
        keys.forEach((k) => {
            const kv = escapeHTML(k + "=" + labels[k]);
            html += '<span class="badge label-badge" title="' + kv + '">' + kv + "</span>";
        });
        return html + "</div>";
    }

    function tableOpen(columns) {
        let html = '<div class="table-responsive"><table class="table neo-table"><thead><tr>';
        columns.forEach((c, i) => {
            html += i === 0 ? '<th style="width: 30px;"></th>' : "<th>" + c + "</th>";
        });
        return html + "</tr></thead><tbody>";
    }

    const tableClose = "</tbody></table></div>";

    function cardPlaceholder(n) {
        let html = '<div class="resource-grid">';
        for (let i = 0; i < n; i++) {
            html += '<div class="neo-card p-3 placeholder-glow">';
            html += '<h6><span class="placeholder col-10"></span></h6>';
            html += '<small class="d-block"><span class="placeholder col-6"></span></small>';
            html += '<small class="d-block"><span class="placeholder col-4"></span></small>';
            html += '<small class="d-block"><span class="placeholder col-5"></span></small>';
            html += "</div>";
        }
        return html + "</div>";
    }

    function resourcePlaceholder() {
        return '<h3 class="mt-3 mb-2 placeholder-glow"><span class="placeholder col-2"></span></h3>' + cardPlaceholder(6);
    }

    function podPlaceholder() {
        let html = tableOpen(podColumns);
        for (let i = 0; i < 6; i++) {
            html += '<tr class="placeholder-glow"><td></td><td><span class="placeholder col-8"></span></td><td><span class="placeholder col-6"></span></td><td><span class="placeholder col-8"></span></td><td><span class="placeholder col-4"></span></td><td><span class="placeholder col-10"></span></td></tr>';
        }
        return html + tableClose;
    }

    function showPlaceholders() {
        resourcesSection.innerHTML = resourcePlaceholder();
        podsSection.innerHTML = podPlaceholder();
    }

    // API kinds are plural ("Ingresses"); the card shows the singular.
    function singular(kind) {
        return kind.endsWith("sses") ? kind.slice(0, -2) : kind.replace(/s$/, "");
    }

    function resourceCard(kind, item) {
        let html = '<div class="neo-card p-3 position-relative" data-kind="' + escapeHTML(kind) + '" data-name="' + escapeHTML(item.name) + '">';
        html += '<i class="bi bi-arrows-fullscreen resource-view-icon position-absolute top-0 end-0 m-2" style="cursor: pointer;"></i>';
        html += '<h6 class="fw-bold pe-4 mb-0" title="' + escapeHTML(item.name) + '">' + escapeHTML(item.name) + "</h6>";
        html += '<div class="resource-kind">' + escapeHTML(singular(kind)) + "</div>";
        if (kind.toLowerCase() === "ingresses" && item.deterministicDNS) {
            // One DNS name per line.
            html += '<small class="text-muted d-block mb-1">' +
                item.deterministicDNS.split(",").map((v) => escapeHTML(v.trim())).join("<br>") +
                "</small>";
        }
        html += '<small class="text-muted d-block">Namespace: ' + escapeHTML(item.namespace) + "</small>";
        html += '<small class="text-muted d-block">Age: ' + escapeHTML(item.age) + "</small>";
        html += '<small class="text-muted d-block">Status: ' + escapeHTML(item.status) + "</small>";
        html += labelBadges(item.labels);
        return html + "</div>";
    }

    function podRow(pod) {
        return '<tr data-name="' + escapeHTML(pod.name) + '">' +
            '<td><i class="bi bi-arrows-fullscreen pod-view-icon" style="cursor: pointer;"></i></td>' +
            "<td><strong>" + escapeHTML(pod.name) + "</strong></td>" +
            '<td><span class="' + statusClass(pod.status) + '">' + escapeHTML(pod.status) + "</span></td>" +
            '<td class="text-break">' + escapeHTML(pod.image || "-") + "</td>" +
            "<td>" + escapeHTML(pod.age) + "</td>" +
            "<td>" + labelBadges(pod.labels) + "</td></tr>";
    }

    function openDescribe(kind, name) {
        const ctx = ctxSelect.value;
        const ns = nsSelect.value;
        describeTitle.textContent = kind + "/" + name;
        describeBody.textContent = "Loading...";
        describeModal.show();

        const params = new URLSearchParams({context: ctx, namespace: ns, kind, name});

        fetch("/api/describe?" + params)
            .then((r) => r.text())
            .then((yaml) => {
                describeBody.textContent = yaml;
            })
            .catch((err) => {
                describeBody.textContent = "Error: " + err.message;
            });
    }

    function updateNamespaces() {
        const ctx = ctxSelect.value;
        nsSelect.innerHTML = "";
        showPlaceholders();
        return fetch("/api/namespaces?context=" + encodeURIComponent(ctx))
            .then(jsonOrThrow)
            .then((namespaces) => {
                namespaces.forEach((ns) => {
                    const opt = document.createElement("option");
                    opt.value = ns;
                    opt.textContent = ns;
                    nsSelect.appendChild(opt);
                });
                return namespaces;
            })
            .catch((err) => {
                resourcesSection.innerHTML =
                    '<p class="text-danger">Error: ' + escapeHTML(err.message) + "</p>";
                podsSection.innerHTML = "";
                return [];
            });
    }

    function loadData() {
        const ctx = ctxSelect.value;
        const ns = nsSelect.value;
        if (!ctx || !ns) return;

        showPlaceholders();

        const params = new URLSearchParams({context: ctx, namespace: ns});

        fetch("/api/resources?" + params)
            .then(jsonOrThrow)
            .then((tables) => {
                let html = "";
                if (!tables || tables.length === 0) {
                    html = '<p class="text-muted">No resources found.</p>';
                } else {
                    const cards = tables.flatMap((t) => t.items.map((item) => resourceCard(t.kind, item)));
                    html += '<h3 class="mt-3 mb-2">Resources <span class="badge bg-dark">' + cards.length + "</span></h3>";
                    html += '<div class="resource-grid">' + cards.join("") + "</div>";
                }
                resourcesSection.innerHTML = html;
                bindResourceClicks();
            })
            .catch((err) => {
                resourcesSection.innerHTML =
                    '<p class="text-danger">Error: ' + escapeHTML(err.message) + "</p>";
            });

        fetch("/api/pods?" + params)
            .then(jsonOrThrow)
            .then((pods) => {
                let html = "";
                if (!pods || pods.length === 0) {
                    html = '<p class="text-muted">No pods found.</p>';
                } else {
                    html = tableOpen(podColumns) + pods.map(podRow).join("") + tableClose;
                }
                podsSection.innerHTML = html;
                bindPodClicks();
            })
            .catch((err) => {
                podsSection.innerHTML =
                    '<p class="text-danger">Error: ' + escapeHTML(err.message) + "</p>";
            });
    }

    function bindResourceClicks() {
        document.querySelectorAll(".resource-view-icon").forEach((icon) => {
            icon.addEventListener("click", function (e) {
                e.stopPropagation();
                const card = this.closest("div[data-name]");
                openDescribe(card.dataset.kind, card.dataset.name);
            });
        });
    }

    function bindPodClicks() {
        document.querySelectorAll(".pod-view-icon").forEach((icon) => {
            icon.addEventListener("click", function (e) {
                e.stopPropagation();
                const name = this.closest("tr").dataset.name;
                window.location.href =
                    "/pod?" + new URLSearchParams({context: ctxSelect.value, namespace: nsSelect.value, name});
            });
        });
    }

    ctxSelect.addEventListener("change", () => {
        updateNamespaces().then((namespaces) => {
            if (namespaces.length > 0) loadData();
        });
    });
    nsSelect.addEventListener("change", loadData);

    // Restore context and namespace from URL parameters
    const urlParams = new URLSearchParams(window.location.search);
    const urlContext = urlParams.get("context");
    const urlNamespace = urlParams.get("namespace");

    if (urlContext) {
        // Try to select the context from URL
        for (let i = 0; i < ctxSelect.options.length; i++) {
            if (ctxSelect.options[i].value === urlContext) {
                ctxSelect.selectedIndex = i;
                break;
            }
        }
    }

    updateNamespaces().then((namespaces) => {
        if (urlNamespace) {
            // Try to select the namespace from URL
            for (let i = 0; i < nsSelect.options.length; i++) {
                if (nsSelect.options[i].value === urlNamespace) {
                    nsSelect.selectedIndex = i;
                    break;
                }
            }
        }
        if (namespaces.length > 0) loadData();
    });
});
