// Javascript to expand row panels and wait until queries
// and panels are fully loaded on the current Grafana
// dashboard

// Base backoff duration in ms
const baseDelayMsecs = 10;

// Define a timer to wait until next try
const timer = ms => new Promise(res => setTimeout(res, ms));

// Panel data
const panelData = selector => [...document.querySelectorAll('[' + selector + ']')].map((e) => ({ "x": e.getBoundingClientRect().x, "y": e.getBoundingClientRect().y, "width": e.getBoundingClientRect().width, "height": e.getBoundingClientRect().height, "title": e.innerText.split('\n')[0], "id": e.getAttribute(selector) }))

/**
 * Semantic Versioning Comparing
 * #see https://semver.org/
 * #see https://stackoverflow.com/a/65687141/456536
 * #see https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/Collator/Collator#options
 */
function semverCompare(a, b) {
    if (a.startsWith(b + "-")) return -1
    if (b.startsWith(a + "-")) return 1
    return a.localeCompare(b, undefined, { numeric: true, sensitivity: "case", caseFirst: "upper" })
}

// Wait for queries to finish and panels to load data
const waitForQueriesAndVisualizations = async (version = '11.3.0', mode = 'default', timeout = 30000) => {
    // Remove v prefix from version
    const ver = version.split('v')[1];

    // Set selector based on version
    let selector;
    if (semverCompare(ver, '11.3.0') === -1) {
        selector = 'data-panelid';
    } else {
        selector = 'data-viz-panel-key'
    }

    // Expand row panels if mode is full
    if (mode === 'full') {
        // For Grafana <= v10
        [...document.getElementsByClassName('dashboard-row--collapsed')].map((e) => e.getElementsByClassName('dashboard-row__title pointer')[0].click());
        // For Grafana > v10 and <= v11
        [...document.querySelectorAll("[data-testid='dashboard-row-container']")].map((e) => [...e.querySelectorAll("[aria-expanded=false]")].map((e) => e.click()));
        // For Grafana >= v11.3
        [...document.querySelectorAll("[aria-label='Expand row']")].map((e) => e.click());
    }

    // Always scroll to bottom of the page
    window.scrollTo(0, document.body.scrollHeight);

    // Panel count should be unchanged for minStableSizeIterations times
    let countStableSizeIterations = 0;
    const minStableSizeIterations = 3;

    // Initialise parameters
    let lastPanels = [];
    let checkCounts = 1;
    const start = Date.now();

    while (Date.now() - start < timeout) {
        // Get current number of rendered panels
        let currentPanels = document.querySelectorAll("[class$='panel-content']");

        // If current panels and last panels are same, increment iterator
        if (lastPanels.length !== 0 && currentPanels.length === lastPanels.length) {
            countStableSizeIterations++;
        } else {
            countStableSizeIterations = 0; // reset the counter
        }

        // If panel count is stable for minStableSizeIterations, return. We assume that
        // the dashboard has loaded with all panels
        if (countStableSizeIterations >= minStableSizeIterations) {
            return panelData(selector);
        }

        // If not, wait and retry
        lastPanels = currentPanels;
        await timer(baseDelayMsecs * 2 ** checkCounts);
        checkCounts++;
    }

    return panelData(selector);
};

// Wait for CSV download button to appear
const waitForCSVDownloadButton = async () => {
    // Initialise parameters
    let checkCounts = 1;
    const start = Date.now();

    // Wait for download button
    while (Date.now() - start < 1000) {
        // Get all buttons on inspect panel
        let buttons = document.querySelectorAll('div[aria-label="Panel inspector Data content"] button[type="button"]');

        // Ensure download CSV button exists in buttons
        for (let i = 0; i < buttons.length; i++) {
            if (buttons[i].innerText === 'Download CSV') {
                buttons[i].click();
                return;
            }
        }

        // If not, wait and retry
        await timer(baseDelayMsecs * 2 ** checkCounts);
        checkCounts++;
    }

    return;
};

// Ensures format data toggle is checked to apply all transformations
const checkFormatDataToggle = async () => {
    // Get all toggles on inspect panel
    let toggles = document.querySelectorAll('div[data-testid="dataOptions"] input#formatted-data-toggle');

    // Ensure format data toggle is checked
    for (let i = 0; i < toggles.length; i++) {
        if (!toggles[i].checked) {
            toggles[i].click();
            return;
        }
    }

    return;
};

// Waits for CSV data to be ready to download
const waitForCSVData = async (version = '11.3.0', timeout = 30000) => {
    // First wait for panel to load data
    await waitForQueriesAndVisualizations(version, 'default', timeout);

    // Ensure format data toggle is checked
    await checkFormatDataToggle();

    // Wait for CSV download button and click it
    await waitForCSVDownloadButton();

    return;
};