document.addEventListener("DOMContentLoaded", function () {
    const ctxSelect = document.getElementById("ctx-select");
    const nsSelect = document.getElementById("ns-select");
    const resourcesSection = document.getElementById("resources-section");
    const podsSection = document.getElementById("pods-section");
    const describeModal = new bootstrap.Modal(document.getElementById("describe-modal"));
    const describeTitle = document.getElementById("describe-modal-title");
    const describeBody = document.getElementById("describe-modal-body");

    function tablePlaceholder() {
        let html = "";
        for (let i = 0; i < 2; i++) {
            html += '<h4 class="mt-3 placeholder-glow"><span class="placeholder col-2"></span></h4>';
            html += '<div class="table-responsive"><table class="table neo-table"><thead><tr><th>Name</th><th>Namespace</th><th>Age</th><th>Status</th></tr></thead><tbody>';
            for (let j = 0; j < 4; j++) {
                html += '<tr class="placeholder-glow"><td><span class="placeholder col-8"></span></td><td><span class="placeholder col-6"></span></td><td><span class="placeholder col-4"></span></td><td><span class="placeholder col-4"></span></td></tr>';
            }
            html += "</tbody></table></div>";
        }
        return html;
    }

    function podPlaceholder() {
        let html = "";
        for (let i = 0; i < 8; i++) {
            html += '<div class="col-md-6 col-lg-4 mb-3"><div class="neo-card p-3 h-100 placeholder-glow">';
            html += '<h6><span class="placeholder col-10"></span></h6>';
            html += '<p class="mb-1"><span class="placeholder col-4"></span></p>';
            html += '<small class="d-block"><span class="placeholder col-6"></span></small>';
            html += '<small class="d-block"><span class="placeholder col-7"></span></small>';
            html += '<small class="d-block"><span class="placeholder col-5"></span></small>';
            html += "</div></div>";
        }
        return html;
    }

    function showPlaceholders() {
        resourcesSection.innerHTML = tablePlaceholder();
        podsSection.innerHTML = podPlaceholder();
    }

    function openDescribe(kind, name) {
        const ctx = ctxSelect.value;
        const ns = nsSelect.value;
        describeTitle.textContent = kind + "/" + name;
        describeBody.textContent = "Loading...";
        describeModal.show();

        const params =
            "context=" + encodeURIComponent(ctx) +
            "&namespace=" + encodeURIComponent(ns) +
            "&kind=" + encodeURIComponent(kind) +
            "&name=" + encodeURIComponent(name);

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
            .then((r) => r.json())
            .then((namespaces) => {
                namespaces.forEach((ns) => {
                    const opt = document.createElement("option");
                    opt.value = ns;
                    opt.textContent = ns;
                    nsSelect.appendChild(opt);
                });
                // After namespaces are loaded, load data with the first namespace
                if (namespaces.length > 0) {
                    loadData();
                }
                return namespaces;
            });
    }

    function loadData() {
        const ctx = ctxSelect.value;
        const ns = nsSelect.value;
        if (!ctx || !ns) return;

        showPlaceholders();

        const params =
            "context=" + encodeURIComponent(ctx) + "&namespace=" + encodeURIComponent(ns);

        fetch("/api/resources?" + params)
            .then((r) => r.json())
            .then((tables) => {
                let html = "";
                if (!tables || tables.length === 0) {
                    html = '<p class="text-muted">No resources found.</p>';
                } else {
                    tables.forEach((t) => {
                        html +=
                            '<h4 class="mt-3">' +
                            t.kind +
                            ' <span class="badge bg-dark">' +
                            t.items.length +
                            "</span></h4>";
                        html +=
                            '<div class="table-responsive"><table class="table neo-table" data-kind="' + t.kind + '"><thead><tr><th style="width: 30px;"></th><th>Name</th><th>Namespace</th><th>Age</th><th>Status</th></tr></thead><tbody>';
                        t.items.forEach((item) => {
                            // Build the name cell content
                            let nameContent = '<strong>' + item.name + '</strong>';
                            if (t.kind === 'ingresses' && item.deterministicDNS) {
                                // Split by comma and display each DNS value on a new line
                                const dnsValues = item.deterministicDNS.split(',').map(v => v.trim());
                                nameContent += '<br><small class="text-muted">';
                                dnsValues.forEach((dns, idx) => {
                                    if (idx > 0) nameContent += '<br>';
                                    nameContent += dns;
                                });
                                nameContent += '</small>';
                            }

                            html +=
                                '<tr data-name="' + item.name + '"><td><i class="bi bi-arrows-fullscreen resource-view-icon" style="cursor: pointer;"></i></td><td>' +
                                nameContent +
                                "</td><td>" +
                                item.namespace +
                                "</td><td>" +
                                item.age +
                                "</td><td>" +
                                item.status +
                                "</td></tr>";
                        });
                        html += "</tbody></table></div>";
                    });
                }
                resourcesSection.innerHTML = html;
                bindRowClicks();
            });

        fetch("/api/pods?" + params)
            .then((r) => r.json())
            .then((pods) => {
                let html = "";
                if (!pods || pods.length === 0) {
                    html =
                        '<div class="col-12"><p class="text-muted">No pods found.</p></div>';
                } else {
                    pods.forEach((pod) => {
                        let statusClass = "";
                        if (pod.status === "Running") statusClass = "status-running";
                        else if (pod.status === "Pending") statusClass = "status-pending";
                        else if (pod.status === "Failed" || pod.status === "CrashLoopBackOff")
                            statusClass = "status-failed";

                        html += '<div class="col-md-6 col-lg-4 mb-3"><div class="neo-card p-3 h-100 position-relative" data-name="' + pod.name + '">';
                        html += '<i class="bi bi-arrows-fullscreen pod-view-icon position-absolute top-0 end-0 m-2" style="cursor: pointer; font-size: 1.2rem;"></i>';
                        html +=
                            '<h6 class="fw-bold" title="' +
                            pod.name +
                            '">' +
                            pod.name +
                            "</h6>";
                        html +=
                            '<p class="mb-1"><span class="' +
                            statusClass +
                            '">' +
                            pod.status +
                            "</span></p>";
                        html +=
                            '<small class="text-muted d-block">Image: ' +
                            (pod.image || "-") +
                            "</small>";
                        html +=
                            '<small class="text-muted d-block">IP: ' +
                            (pod.ip || "-") +
                            "</small>";
                        html +=
                            '<small class="text-muted d-block">Node: ' +
                            (pod.node || "-") +
                            "</small>";
                        html +=
                            '<small class="text-muted d-block">Age: ' +
                            pod.age +
                            "</small>";
                        html += "</div></div>";
                    });
                }
                podsSection.innerHTML = html;
                bindPodClicks();
            });
    }

    function bindRowClicks() {
        document.querySelectorAll(".resource-view-icon").forEach((icon) => {
            icon.addEventListener("click", function (e) {
                e.stopPropagation();
                const row = this.closest("tr");
                const kind = row.closest("table").dataset.kind;
                const name = row.dataset.name;
                openDescribe(kind, name);
            });
        });
    }

    function bindPodClicks() {
        document.querySelectorAll(".pod-view-icon").forEach((icon) => {
            icon.addEventListener("click", function (e) {
                e.stopPropagation();
                const card = this.closest("div[data-name]");
                const ctx = ctxSelect.value;
                const ns = nsSelect.value;
                const name = card.dataset.name;
                window.location.href =
                    "/pod?context=" + encodeURIComponent(ctx) +
                    "&namespace=" + encodeURIComponent(ns) +
                    "&name=" + encodeURIComponent(name);
            });
        });
    }

    ctxSelect.addEventListener("change", updateNamespaces);
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

    updateNamespaces().then(() => {
        if (urlNamespace) {
            // Try to select the namespace from URL
            for (let i = 0; i < nsSelect.options.length; i++) {
                if (nsSelect.options[i].value === urlNamespace) {
                    nsSelect.selectedIndex = i;
                    // loadData was already called by updateNamespaces,
                    // but we need to call it again with the correct namespace
                    loadData();
                    return;
                }
            }
        }
        // If no URL namespace or not found, loadData was already called by updateNamespaces
    });
});
