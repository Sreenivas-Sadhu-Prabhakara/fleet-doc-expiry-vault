/* Fleet Document Expiry Vault — local interface.
   Plain DOM, no framework, no inline handlers. Every call goes to this same
   binary on the loopback address; nothing leaves the machine. */
(function () {
  "use strict";

  /* ---------- theme ---------- */

  var root = document.documentElement;

  function storedTheme() {
    try { return localStorage.getItem("omsme-theme"); } catch (e) { return null; }
  }
  function currentTheme() {
    var set = root.getAttribute("data-theme");
    if (set) { return set; }
    return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
  }
  function paintThemer(button) {
    if (!button) { return; }
    var dark = currentTheme() === "dark";
    button.textContent = dark ? "☀ Light" : "☾ Dark";
    button.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
  }
  (function wireTheme() {
    var button = document.querySelector("[data-themer]");
    if (!button) { return; }
    paintThemer(button);
    button.addEventListener("click", function () {
      var next = currentTheme() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* private window */ }
      paintThemer(button);
    });
    if (window.matchMedia) {
      window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (!storedTheme()) { paintThemer(button); }
      });
    }
  }());

  /* ---------- small helpers ---------- */

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) { node.className = className; }
    if (text !== undefined && text !== null) { node.textContent = String(text); }
    return node;
  }

  function clear(node) {
    while (node.firstChild) { node.removeChild(node.firstChild); }
  }

  function byId(id) { return document.getElementById(id); }

  var flashTimer = null;
  function flash(message, level) {
    var box = byId("flash");
    box.textContent = message;
    box.setAttribute("data-level", level || "info");
    box.hidden = false;
    if (flashTimer) { window.clearTimeout(flashTimer); }
    flashTimer = window.setTimeout(function () { box.hidden = true; }, 7000);
  }

  function api(path, options) {
    return window.fetch(path, options || {}).then(function (response) {
      var type = response.headers.get("content-type") || "";
      if (type.indexOf("application/json") < 0) {
        if (!response.ok) { throw new Error("Request failed (" + response.status + ")"); }
        return null;
      }
      return response.json().then(function (body) {
        if (!response.ok) { throw new Error((body && body.error) || "Request failed (" + response.status + ")"); }
        return body;
      });
    });
  }

  function send(path, method, payload) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
  }

  function fail(err) { flash(err && err.message ? err.message : String(err), "bad"); }

  // "2027-03-31" -> "31-03-2027", the way it is printed on the paper.
  function pretty(iso) {
    if (!iso) { return "—"; }
    var parts = String(iso).split("-");
    if (parts.length !== 3) { return String(iso); }
    return parts[2] + "-" + parts[1] + "-" + parts[0];
  }

  function plate(reg) {
    var text = String(reg || "").toUpperCase().replace(/[^A-Z0-9]/g, "");
    var groups = text.match(/\d+|[A-Z]+/g) || [];
    if (groups.length < 2 || groups.length > 4) { return text; }
    return groups.join(" ");
  }

  function statusChip(status) {
    var chip = el("span", "chip " + (status.band || "grey"));
    if (status.kind === "expired") {
      chip.textContent = "Expired";
    } else if (status.kind === "dueSoon") {
      chip.textContent = status.daysLeft === 0 ? "Today" : "Due soon";
    } else if (status.kind === "valid") {
      chip.textContent = "Valid";
    } else {
      chip.textContent = "No date";
    }
    return chip;
  }

  function dayCell(status) {
    var cell = el("td", "num");
    if (status.kind === "expired") {
      cell.textContent = "−" + status.daysOverdue;
      cell.title = "Overdue by " + status.daysOverdue + " days";
    } else if (status.kind === "unknown") {
      cell.textContent = "—";
    } else {
      cell.textContent = String(status.daysLeft);
      cell.title = status.daysLeft + " days left";
    }
    return cell;
  }

  /* ---------- state ---------- */

  var state = { vehicles: [], drivers: [], documents: [], kinds: [], tiers: [30, 15, 3], today: "" };

  function kindLabel(kind) {
    for (var i = 0; i < state.kinds.length; i += 1) {
      if (state.kinds[i].kind === kind) { return state.kinds[i].label; }
    }
    return kind;
  }

  function kindInfo(kind) {
    for (var i = 0; i < state.kinds.length; i += 1) {
      if (state.kinds[i].kind === kind) { return state.kinds[i]; }
    }
    return null;
  }

  function driverName(id) {
    for (var i = 0; i < state.drivers.length; i += 1) {
      if (state.drivers[i].id === id) { return state.drivers[i].name; }
    }
    return id;
  }

  function asOf() {
    var value = byId("asof").value;
    return value ? value : state.today;
  }

  function asOfQuery() {
    return "?asOf=" + encodeURIComponent(asOf());
  }

  function loadState() {
    return api("api/state").then(function (body) {
      state = body;
      // The top bar has room for the file name; the whole path goes in the
      // settings panel, where it can be read and copied.
      var slash = Math.max(body.dataFile.lastIndexOf("/"), body.dataFile.lastIndexOf("\\"));
      byId("data-file").textContent = body.dataFile.slice(slash + 1);
      byId("data-file").title = body.dataFile;
      byId("data-file-full").textContent = body.dataFile;
      byId("tier-text").textContent = body.tiers.join(", ");
      if (!byId("asof").value) { byId("asof").value = body.today; }
      renderKindSelects();
      renderVehicleSelects();
      renderVehicleList();
      renderDriverList();
      renderPolicy();
      return body;
    });
  }

  /* ---------- tabs ---------- */

  var tabs = Array.prototype.slice.call(document.querySelectorAll("[data-tab]"));

  function showTab(name) {
    tabs.forEach(function (button) {
      button.setAttribute("aria-pressed", button.getAttribute("data-tab") === name ? "true" : "false");
    });
    ["board", "run", "vehicles", "drivers", "documents", "renewals", "data"].forEach(function (key) {
      byId("panel-" + key).hidden = key !== name;
    });
    if (name === "board") { refreshBoard(); }
    if (name === "run") { refreshRun(); }
    if (name === "drivers") { refreshLicences(); }
    if (name === "renewals") { refreshChecklists(); }
  }

  tabs.forEach(function (button) {
    button.addEventListener("click", function () { showTab(button.getAttribute("data-tab")); });
  });

  /* ---------- fleet board ---------- */

  function renderCounts(counts) {
    var box = byId("board-counts");
    clear(box);
    [
      ["red", counts.expired, "Expired"],
      ["amber", counts.dueSoon, "Due soon"],
      ["green", counts.valid, "Valid"],
      ["grey", counts.unknown, "No date"]
    ].forEach(function (row) {
      var card = el("div", "c " + row[0]);
      card.appendChild(el("div", "n", row[1]));
      card.appendChild(el("div", "l", row[2]));
      box.appendChild(card);
    });
  }

  function boardRow(row) {
    var tr = el("tr", row.status.kind === "expired" ? "gone" : "");

    var statusCell = el("td");
    statusCell.appendChild(statusChip(row.status));
    tr.appendChild(statusCell);

    var subject = el("td", "plate");
    subject.appendChild(document.createTextNode(row.subjectLabel));
    if (row.detail) { subject.appendChild(el("span", "sub2", row.detail)); }
    tr.appendChild(subject);

    var doc = el("td");
    doc.appendChild(document.createTextNode(row.label));
    if (row.note) { doc.appendChild(el("span", "sub2", row.note)); }
    tr.appendChild(doc);

    tr.appendChild(el("td", "plate", row.number || "—"));
    tr.appendChild(el("td", "", pretty(row.expiresOn)));
    tr.appendChild(dayCell(row.status));
    tr.appendChild(el("td", "ref", row.fileRef || "—"));
    return tr;
  }

  function refreshBoard() {
    return api("api/board" + asOfQuery()).then(function (body) {
      var body2 = byId("board-table").tBodies[0];
      clear(body2);
      body.rows.forEach(function (row) { body2.appendChild(boardRow(row)); });
      renderCounts(body.counts);
      byId("board-empty").hidden = body.rows.length > 0;
      byId("board-table").hidden = body.rows.length === 0;
    }).catch(fail);
  }

  byId("asof").addEventListener("change", function () {
    refreshBoard();
    if (!byId("panel-run").hidden) { refreshRun(); }
    if (!byId("panel-drivers").hidden) { refreshLicences(); }
  });

  /* ---------- can it run today ---------- */

  function reasonItem(item) {
    var li = el("li");
    li.appendChild(el("span", "what", item.label + ": "));
    li.appendChild(document.createTextNode(item.reason));
    return li;
  }

  function reasonBlock(title, items) {
    var wrap = document.createDocumentFragment();
    if (!items || items.length === 0) { return wrap; }
    wrap.appendChild(el("h3", "sub", title));
    var list = el("ul", "reasons");
    items.forEach(function (item) { list.appendChild(reasonItem(item)); });
    wrap.appendChild(list);
    return wrap;
  }

  function refreshRun() {
    var select = byId("run-vehicle");
    byId("run-empty").hidden = state.vehicles.length > 0;
    if (state.vehicles.length === 0) {
      clear(byId("run-answer"));
      return Promise.resolve();
    }
    var regNo = select.value || state.vehicles[0].regNo;
    return api("api/can-run?regNo=" + encodeURIComponent(regNo) + "&asOf=" + encodeURIComponent(asOf()))
      .then(function (body) {
        var out = byId("run-answer");
        clear(out);

        var card = el("div", "verdict " + (body.verdict.canRun ? "yes" : "no"));
        card.appendChild(el("div", "k", "As of " + pretty(body.verdict.asOf)));
        card.appendChild(el("div", "v", body.verdict.headline));
        card.appendChild(el("div", "sub", body.verdict.detail));
        out.appendChild(card);

        out.appendChild(reasonBlock("Expired and blocking", body.verdict.blocking));
        out.appendChild(reasonBlock("Not on file", body.verdict.missing));
        out.appendChild(reasonBlock("Expired, set not to block", body.verdict.reported));
        out.appendChild(reasonBlock("Coming up for renewal", body.verdict.expiring));

        if (body.nextDue) {
          var next = el("p", "note");
          next.textContent = "Next thing to deal with: " + kindLabel(body.nextDue.kind)
            + ", valid until " + pretty(body.nextDue.expiresOn) + ".";
          out.appendChild(next);
        }

        var heading = el("h3", "sub", "Every document on file for " + plate(body.vehicle.regNo));
        out.appendChild(heading);
        var wrap = el("div", "tablewrap");
        var table = el("table", "breakdown board");
        var head = table.createTHead().insertRow();
        ["Status", "Vehicle / driver", "Document", "Number", "Expires", "Days", "Scan"].forEach(function (title, i) {
          var th = el("th", i === 5 ? "num" : "", title);
          head.appendChild(th);
        });
        var tbody = table.createTBody();
        body.rows.forEach(function (row) { tbody.appendChild(boardRow(row)); });
        wrap.appendChild(table);
        out.appendChild(wrap);
      }).catch(fail);
  }

  byId("run-vehicle").addEventListener("change", refreshRun);

  /* ---------- vehicles ---------- */

  function renderVehicleSelects() {
    var run = byId("run-vehicle");
    var previous = run.value;
    clear(run);
    state.vehicles.forEach(function (vehicle) {
      var option = el("option", "", plate(vehicle.regNo) + (vehicle.make ? " · " + vehicle.make : ""));
      option.value = vehicle.regNo;
      run.appendChild(option);
    });
    if (previous) { run.value = previous; }

    var subject = byId("doc-subject");
    var previousSubject = subject.value;
    clear(subject);
    var kind = byId("doc-kind").value;
    var info = kindInfo(kind);
    if (info && info.belongsToDriver) {
      state.drivers.forEach(function (driver) {
        var option = el("option", "", driver.name + " · " + driver.id);
        option.value = "driver:" + driver.id;
        subject.appendChild(option);
      });
    } else {
      state.vehicles.forEach(function (vehicle) {
        var option = el("option", "", plate(vehicle.regNo) + (vehicle.make ? " · " + vehicle.make : ""));
        option.value = "vehicle:" + vehicle.regNo;
        subject.appendChild(option);
      });
    }
    if (previousSubject) { subject.value = previousSubject; }
    byId("document-empty").hidden = subject.options.length > 0;
  }

  function recordRow(title, meta, actions) {
    var row = el("div", "record");
    row.appendChild(el("span", "who2", title));
    row.appendChild(el("span", "meta", meta));
    var acts = el("div", "acts");
    actions.forEach(function (action) {
      var button = el("button", action.danger ? "danger" : "", action.label);
      button.type = "button";
      button.addEventListener("click", action.run);
      acts.appendChild(button);
    });
    row.appendChild(acts);
    return row;
  }

  function renderVehicleList() {
    var box = byId("vehicle-list");
    clear(box);
    if (state.vehicles.length === 0) {
      box.appendChild(el("p", "empty", "No vehicles yet."));
      return;
    }
    var wrap = el("div", "records");
    state.vehicles.forEach(function (vehicle) {
      var count = state.documents.filter(function (doc) {
        return doc.vehicleRegNo === vehicle.regNo;
      }).length;
      var meta = [
        vehicle.type === "goods" ? "Goods carriage" : "Passenger vehicle",
        vehicle.make || null,
        vehicle.ownerName || null,
        count + (count === 1 ? " document" : " documents")
      ].filter(Boolean).join(" · ");
      wrap.appendChild(recordRow(plate(vehicle.regNo), meta, [
        {
          label: "Edit", run: function () {
            byId("v-regno").value = plate(vehicle.regNo);
            byId("v-type").value = vehicle.type;
            byId("v-make").value = vehicle.make;
            byId("v-owner").value = vehicle.ownerName;
            byId("v-regno").focus();
          }
        },
        {
          label: "Check today", run: function () {
            byId("run-vehicle").value = vehicle.regNo;
            showTab("run");
          }
        },
        {
          label: "Remove", danger: true, run: function () {
            if (!window.confirm("Remove " + plate(vehicle.regNo) + " and all its documents?")) { return; }
            api("api/vehicle?regNo=" + encodeURIComponent(vehicle.regNo), { method: "DELETE" })
              .then(function () {
                flash(plate(vehicle.regNo) + " removed.", "ok");
                return loadState().then(refreshBoard);
              }).catch(fail);
          }
        }
      ]));
    });
    box.appendChild(wrap);
  }

  byId("vehicle-form").addEventListener("submit", function (event) {
    event.preventDefault();
    send("api/vehicle", "PUT", {
      regNo: byId("v-regno").value,
      type: byId("v-type").value,
      make: byId("v-make").value,
      ownerName: byId("v-owner").value
    }).then(function (saved) {
      flash(plate(saved.regNo) + " saved.", "ok");
      byId("vehicle-form").reset();
      return loadState().then(refreshBoard);
    }).catch(fail);
  });

  /* ---------- drivers ---------- */

  function renderDriverList() {
    var box = byId("driver-list");
    clear(box);
    if (state.drivers.length === 0) { return; }
    var wrap = el("div", "records");
    state.drivers.forEach(function (driver) {
      var meta = [driver.id, driver.phone || null].filter(Boolean).join(" · ");
      wrap.appendChild(recordRow(driver.name, meta, [
        {
          label: "Edit", run: function () {
            byId("d-id").value = driver.id;
            byId("d-name").value = driver.name;
            byId("d-phone").value = driver.phone;
            byId("d-name").focus();
          }
        },
        {
          label: "Remove", danger: true, run: function () {
            if (!window.confirm("Remove " + driver.name + " and their licence record?")) { return; }
            api("api/driver?id=" + encodeURIComponent(driver.id), { method: "DELETE" })
              .then(function () {
                flash(driver.name + " removed.", "ok");
                return loadState().then(refreshLicences);
              }).catch(fail);
          }
        }
      ]));
    });
    box.appendChild(wrap);
  }

  byId("driver-form").addEventListener("submit", function (event) {
    event.preventDefault();
    send("api/driver", "PUT", {
      id: byId("d-id").value,
      name: byId("d-name").value,
      phone: byId("d-phone").value
    }).then(function (saved) {
      flash(saved.name + " saved as " + saved.id + ".", "ok");
      byId("driver-form").reset();
      byId("d-id").value = "";
      return loadState().then(refreshLicences);
    }).catch(fail);
  });

  function refreshLicences() {
    return api("api/licences" + asOfQuery()).then(function (body) {
      var tbody = byId("licence-table").tBodies[0];
      clear(tbody);
      body.rows.forEach(function (row) {
        var tr = el("tr", row.status.kind === "expired" ? "gone" : "");
        var statusCell = el("td");
        statusCell.appendChild(statusChip(row.status));
        tr.appendChild(statusCell);
        tr.appendChild(el("td", "", row.subjectLabel));
        tr.appendChild(el("td", "plate", row.detail || "—"));
        tr.appendChild(el("td", "plate", row.number || "—"));
        tr.appendChild(el("td", "", pretty(row.expiresOn)));
        tr.appendChild(dayCell(row.status));
        tr.appendChild(el("td", "ref", row.fileRef || "—"));
        tbody.appendChild(tr);
      });
      byId("licence-empty").hidden = body.rows.length > 0;
      byId("licence-table").hidden = body.rows.length === 0;
    }).catch(fail);
  }

  /* ---------- documents ---------- */

  function renderKindSelects() {
    var select = byId("doc-kind");
    var previous = select.value;
    clear(select);
    state.kinds.forEach(function (info) {
      var option = el("option", "", info.label);
      option.value = info.kind;
      select.appendChild(option);
    });
    if (previous) { select.value = previous; }

    var checklist = byId("checklist-kind");
    var previousChecklist = checklist.value;
    clear(checklist);
    var all = el("option", "", "Every document (one printable sheet)");
    all.value = "";
    checklist.appendChild(all);
    state.kinds.forEach(function (info) {
      var option = el("option", "", info.label);
      option.value = info.kind;
      checklist.appendChild(option);
    });
    checklist.value = previousChecklist || "";
    renderValidityNote();
  }

  function renderValidityNote() {
    var info = kindInfo(byId("doc-kind").value);
    byId("doc-validity-note").textContent = info ? info.validityNote : "";
    byId("doc-suggest").disabled = !info || (info.validity.firstIssueMonths === 0 && info.validity.renewalMonths === 0);
  }

  byId("doc-kind").addEventListener("change", function () {
    renderValidityNote();
    renderVehicleSelects();
  });

  byId("doc-suggest").addEventListener("click", function () {
    var issued = byId("doc-issued").value;
    if (!issued) {
      flash("Enter the issue date first — the suggestion is counted from it.", "bad");
      return;
    }
    api("api/suggest-expiry?kind=" + encodeURIComponent(byId("doc-kind").value)
      + "&issuedOn=" + encodeURIComponent(issued)
      + "&firstIssue=" + (byId("doc-first").checked ? "true" : "false"))
      .then(function (body) {
        if (!body.found) {
          flash("No default validity period for this document — enter the date from the paper.", "bad");
          return;
        }
        byId("doc-expires").value = body.suggested;
        flash("Suggested " + pretty(body.suggested) + ". Check it against the certificate before saving.", "info");
      }).catch(fail);
  });

  byId("document-form").addEventListener("submit", function (event) {
    event.preventDefault();
    var subject = byId("doc-subject").value;
    if (!subject) {
      flash("Add a vehicle or a driver first — a document has to belong to something.", "bad");
      return;
    }
    var split = subject.indexOf(":");
    var owner = subject.slice(0, split);
    var key = subject.slice(split + 1);
    send("api/document", "PUT", {
      vehicleRegNo: owner === "vehicle" ? key : "",
      driverId: owner === "driver" ? key : "",
      kind: byId("doc-kind").value,
      number: byId("doc-number").value,
      issuedOn: byId("doc-issued").value,
      expiresOn: byId("doc-expires").value,
      fileRef: byId("doc-fileref").value
    }).then(function (saved) {
      flash(kindLabel(saved.kind) + " saved, valid until " + pretty(saved.expiresOn) + ".", "ok");
      byId("doc-number").value = "";
      byId("doc-issued").value = "";
      byId("doc-expires").value = "";
      byId("doc-fileref").value = "";
      byId("doc-first").checked = false;
      return loadState().then(refreshBoard);
    }).catch(fail);
  });

  /* ---------- renewal checklists ---------- */

  function checklistSheet(list) {
    var sheet = el("div", "sheet");
    sheet.appendChild(el("h3", "", "Renewing: " + list.label));
    sheet.appendChild(el("p", "where", list.where));
    if (list.forms && list.forms.length) {
      sheet.appendChild(el("h4", "", "Forms"));
      var forms = el("ul");
      list.forms.forEach(function (form) { forms.appendChild(el("li", "", form)); });
      sheet.appendChild(forms);
    }
    sheet.appendChild(el("h4", "", "Take with you"));
    var items = el("ul", "tick");
    list.items.forEach(function (item) { items.appendChild(el("li", "", item)); });
    sheet.appendChild(items);
    sheet.appendChild(el("p", "note", list.note));
    return sheet;
  }

  function refreshChecklists() {
    var kind = byId("checklist-kind").value;
    var path = kind ? "api/checklist?kind=" + encodeURIComponent(kind) : "api/checklist";
    return api(path).then(function (lists) {
      var box = byId("checklist-body");
      clear(box);
      lists.forEach(function (list) { box.appendChild(checklistSheet(list)); });
    }).catch(fail);
  }

  byId("checklist-kind").addEventListener("change", refreshChecklists);
  byId("checklist-print").addEventListener("click", function () { window.print(); });

  /* ---------- policy and data ---------- */

  function renderPolicy() {
    var box = byId("policy-blocking");
    clear(box);
    var grid = el("div", "toggles");
    state.kinds.forEach(function (info) {
      if (info.belongsToDriver) { return; }
      var label = el("label");
      var input = document.createElement("input");
      input.type = "checkbox";
      input.checked = info.blocks;
      input.setAttribute("data-blocking", info.kind);
      label.appendChild(input);
      label.appendChild(document.createTextNode("An expired " + info.label + " stops the vehicle"));
      grid.appendChild(label);
    });
    box.appendChild(grid);

    var tbody = byId("policy-validity").tBodies[0];
    clear(tbody);
    state.kinds.forEach(function (info) {
      var tr = el("tr");
      tr.appendChild(el("td", "", info.label));
      [["first", info.validity.firstIssueMonths], ["renewal", info.validity.renewalMonths]].forEach(function (pair) {
        var cell = el("td", "num");
        var input = document.createElement("input");
        input.type = "number";
        input.min = "0";
        input.max = "600";
        input.step = "1";
        input.value = String(pair[1]);
        input.setAttribute("data-validity", info.kind);
        input.setAttribute("data-leg", pair[0]);
        input.setAttribute("aria-label", info.label + " validity in months, " + pair[0]);
        cell.appendChild(input);
        tr.appendChild(cell);
      });
      tr.appendChild(el("td", "basis", info.validityNote));
      tbody.appendChild(tr);
    });
  }

  byId("policy-save").addEventListener("click", function () {
    var blocking = {};
    Array.prototype.slice.call(document.querySelectorAll("[data-blocking]")).forEach(function (input) {
      blocking[input.getAttribute("data-blocking")] = input.checked;
    });
    var validity = {};
    Array.prototype.slice.call(document.querySelectorAll("[data-validity]")).forEach(function (input) {
      var kind = input.getAttribute("data-validity");
      if (!validity[kind]) { validity[kind] = { firstIssueMonths: 0, renewalMonths: 0 }; }
      var months = parseInt(input.value, 10);
      if (isNaN(months) || months < 0) { months = 0; }
      if (input.getAttribute("data-leg") === "first") {
        validity[kind].firstIssueMonths = months;
      } else {
        validity[kind].renewalMonths = months;
      }
    });
    send("api/policy", "PUT", { blocking: blocking, validity: validity }).then(function () {
      flash("Settings saved.", "ok");
      return loadState().then(refreshBoard);
    }).catch(fail);
  });

  byId("policy-reset").addEventListener("click", function () {
    send("api/policy", "PUT", {
      blocking: { RC: false, Insurance: true, Fitness: true, Permit: true, PUC: true, RoadTax: false },
      validity: {
        RC: { firstIssueMonths: 0, renewalMonths: 0 },
        Insurance: { firstIssueMonths: 12, renewalMonths: 12 },
        Fitness: { firstIssueMonths: 24, renewalMonths: 12 },
        Permit: { firstIssueMonths: 60, renewalMonths: 60 },
        PUC: { firstIssueMonths: 12, renewalMonths: 6 },
        RoadTax: { firstIssueMonths: 0, renewalMonths: 0 },
        DrivingLicence: { firstIssueMonths: 0, renewalMonths: 0 }
      }
    }).then(function () {
      flash("Back to the defaults.", "ok");
      return loadState().then(refreshBoard);
    }).catch(fail);
  });

  byId("import-run").addEventListener("click", function () {
    var picker = byId("import-file");
    if (!picker.files || picker.files.length === 0) {
      flash("Choose a CSV file first.", "bad");
      return;
    }
    var mode = byId("import-mode").value;
    if (mode === "replace" && !window.confirm("Replace every vehicle, driver and document with the contents of this file?")) {
      return;
    }
    picker.files[0].text().then(function (text) {
      return api("api/import.csv?mode=" + encodeURIComponent(mode), {
        method: "POST",
        headers: { "Content-Type": "text/csv" },
        body: text
      });
    }).then(function (report) {
      var box = byId("import-report");
      clear(box);
      box.appendChild(el("p", "note", "Imported (" + report.mode + "): " + report.vehicles
        + " vehicles, " + report.drivers + " drivers, " + report.documents + " documents. "
        + report.counts.expired + " expired, " + report.counts.dueSoon + " due soon."));
      if (report.warnings.length) {
        box.appendChild(el("p", "empty", "Rows the vault could not use:"));
        var list = el("ul", "warn-list");
        report.warnings.forEach(function (warning) { list.appendChild(el("li", "", warning)); });
        box.appendChild(list);
      }
      flash("Import finished.", "ok");
      return loadState().then(refreshBoard);
    }).catch(fail);
  });

  /* ---------- go ---------- */

  loadState().then(refreshBoard).catch(fail);
}());
