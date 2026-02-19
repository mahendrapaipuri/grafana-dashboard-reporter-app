// Javascript to expand row panels and wait until queries
// and panels are fully loaded on the current Grafana
// dashboard

// Fallback version string
const fallbackVersion = '11.3.0'

// Base backoff duration in ms
const baseDelayMsecs = 10;

// Define a timer to wait until next try
const timer = ms => new Promise(res => setTimeout(res, ms));

// Wait for element to either appear or disappear on DOM
// Seems like this approach does not work well. Need to dissect more
// on why!!
const waitForElement = async (selector, appear=true) => {
    // Initialise parameters
    let checkCounts = 1;
    const start = Date.now();

    // Wait for download button
    while (Date.now() - start < 1000) {
        // Query for element
        const element = document.querySelectorAll(selector);

        // If appear is true check for element existence
        if (appear) {
            if (element) {
                return;
            }
        } else {
            if (!element) {
                return;
            }
        }

        // If not, wait and retry
        await timer(baseDelayMsecs * 2 ** checkCounts);
        checkCounts++;
    }

    return;
};

// Panel data
const panelData = async (selector) => {
    // Get all panels
    const panelElements = document.querySelectorAll('[' + selector + ']');

    // Get panel details
    let panels = [];
    for (let e of panelElements) {
        // Get panel ID
        let id = e.getAttribute(selector);

        // Now click all the panels show-on-hover buttons to get View options
        [...e.querySelectorAll('[title="Menu"]')].map((ee) => ee.click());

        // Wait for dialog to appear
        await timer(100);

        // Now fetch all the hrefs from the resulting dialogs for each panel
        const viewItems = document.querySelectorAll('[data-testid*="data-testid Panel menu item View"]');

        // This href attribute exists only after Grafana 11.3.0 (I guess). It does not exist
        // at least in Grafana 11.0.0
        if (viewItems.length === 1 && viewItems[0].hasAttribute('href')) {
            let url =  new URL(viewItems[0].href);

            // Get viewPanel query parameter
            id = url.searchParams.get('viewPanel') || id;
        }

        // Get panel dimensions
        panels.push({ "x": e.getBoundingClientRect().x, "y": e.getBoundingClientRect().y, "width": e.getBoundingClientRect().width, "height": e.getBoundingClientRect().height, "title": e.innerText.split('\n')[0], "id": id });

        // Click show-on-hover again to close dialog
        [...e.querySelectorAll('[title="Menu"]')].map((ee) => ee.click());

        // Wait for dialog to disappear
        await timer(100);
    }
    
    return panels
}

/**
 * Semantic Versioning Comparing
 * #see https://semver.org/
 * #see https://stackoverflow.com/a/65687141/456536
 * #see https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/Collator/Collator#options
 * 
 * Seems like Grafana uses "-" for pre-releases and "+" for post releases (bug fixes)
 */
function semverCompare(a, b) {
    // Pre-releases
    if (a.startsWith(b + "-")) {return -1}
    if (b.startsWith(a + "-")) {return 1}

    // Post releases
    if (a.startsWith(b + "+")) {return 1}
    if (b.startsWith(a + "+")) {return -1}
    return a.localeCompare(b, undefined, { numeric: true, sensitivity: "case", caseFirst: "upper" })
}

// Wait for queries to finish and panels to load data
const waitForQueriesAndVisualizations = async (version = `v${fallbackVersion}`, mode = 'default', timeout = 30000) => {
    // Remove v prefix from version
    let ver = version.split('v')[1] || '0.0.0';

    // Seems like Grafana is CAPABLE of sending zero version string
    // on backend plugin. In that case attempt to get version from 
    // frontend boot data
    if (semverCompare(ver, '0.0.0') === 0) {
        ver = grafanaBootData?.settings?.buildInfo?.version || fallbackVersion 
    }

    // Set selector based on version
    let selector;
    if (semverCompare(ver, '11.3.0') === -1) {
        selector = 'data-panelid';
    } else {
        selector = 'data-viz-panel-key'
    }

    // Hide dock menu if present
    let dockMenu = document.getElementById('dock-menu-button');
    if (dockMenu !== null) {
        dockMenu.click();
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
            // Instead of checking for literal "Download CSV", check
            // if button text contains CSV. This ensures that it will
            // work for Grafana instances that are not in English.
            if (buttons[i].innerText.includes('CSV')) {
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

// Expands data options tab
const expandDataOptionsTab = async () => {
    // Get data options tab node
    let tabs = document.querySelectorAll('div[role="dialog"] button[aria-expanded=false]');

    // Ensure data options tab is expanded
    for (let i = 0; i < tabs.length; i++) {
        tabs[i].click();
    }

    return;
};

// Ensures format data toggle is checked to apply all transformations
const checkFormatDataToggle = async () => {
    // Expand data options tab
    await expandDataOptionsTab();

    // Get all toggles on inspect panel
    let toggles = document.querySelectorAll('div[data-testid="dataOptions"] input:not(#excel-toggle)');

    // Ensure all data format toggles are checked
    for (let i = 0; i < toggles.length; i++) {
        if (!toggles[i].checked) {
            toggles[i].click();
        }
    }

    return;
};

// Opens Inspect Data tab
const openInspectDataTab = async () => {
    // Click the top right menu
    [...document.querySelectorAll('[title="Menu"]')].map((ee) => ee.click());

    // Wait for dialog to appear
    await timer(100);

    // Create a mouse event to hover on Inspect menu item
    var hoverInspectEvent = new MouseEvent('mouseover', {
        'view': window,
        'bubbles': true,
        'cancelable': true
    }); 

    // Dispatch event on window
    let inspectItems = document.querySelectorAll('[data-testid*="data-testid Panel menu item Inspect"]');

    // Dispatch event on inspect item
    for (let i = 0; i < inspectItems.length; i++) {
        inspectItems[i].dispatchEvent(hoverInspectEvent);
    }

    // Wait for dispatch event
    await timer(100);

    // Click data menu item
    [...document.querySelectorAll('[data-testid*="data-testid Panel menu item Data"]')].map((ee) => ee.click());

    return;
};

// Waits for CSV data to be ready to download
const waitForCSVData = async (version = `v${fallbackVersion}`, timeout = 30000) => {
    // First wait for panel to load data
    await waitForQueriesAndVisualizations(version, 'default', timeout);

    // Open inspect tab
    await openInspectDataTab();

    // Ensure format data toggle is checked
    await checkFormatDataToggle();

    // Wait for CSV download button and click it
    await waitForCSVDownloadButton();

    return;
};
