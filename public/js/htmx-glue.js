(() => {
    const fieldSelector = 'input:not([type=hidden], [type=file], [type=submit]), textarea, select';
    const requests = new WeakMap();
    const regions = new WeakMap();
    const elementRequests = new WeakMap();
    const latestRequests = new WeakMap();

    const showBanner = (text) => {
        const banner = document.getElementById('page-error');
        if (banner) {
            banner.textContent = text;
            banner.scrollIntoView({ block: 'nearest' });
        }
    };

    const setPageStatus = (text) => {
        const status = document.getElementById('page-status');
        if (status) {
            status.textContent = text;
        }
    };

    // The button or submit that started the action being handled, which is gone once the current event ends.
    let trigger = null;
    const noteTrigger = (button) => {
        trigger = button;
        setTimeout(() => {
            trigger = null;
        });
    };

    const isChoice = (field) => field.type === 'checkbox' || field.type === 'radio';
    const fieldKey = (field) => (isChoice(field) ? `${field.name}=${field.value}` : field.name);
    const fieldState = (field) => (isChoice(field) ? field.checked : field.value);
    const fieldsIn = (region) => [...region.querySelectorAll(fieldSelector)].filter((field) => field.name);
    const setFieldState = (field, state) => {
        if (isChoice(field)) {
            field.checked = state;
        } else {
            field.value = state;
        }
    };
    const regionState = (region) => {
        if (!regions.has(region)) {
            regions.set(region, { active: new Set(), dropped: false });
        }
        return regions.get(region);
    };

    const markBusy = (element) => {
        elementRequests.set(element, (elementRequests.get(element) ?? 0) + 1);
        element.setAttribute('aria-busy', 'true');
    };

    const clearBusy = (element) => {
        const remaining = (elementRequests.get(element) ?? 1) - 1;
        elementRequests.set(element, remaining);
        if (remaining === 0) {
            element.removeAttribute('aria-busy');
        }
    };

    document.addEventListener('htmx:before:request', (event) => {
        const region = event.target.closest?.('[data-busy-region]');
        const request = { start: performance.now(), trigger, timedOut: () => performance.now() - request.start >= htmx.config.defaultTimeout - 100, element: event.target, region, values: null, typed: new Map(), active: null, dropped: false };
        requests.set(event.detail.ctx, request);
        // htmx logs a rejection with a reason to the console and skips one without, and it aborts a replaced request
        // and a timed-out one the same way. Only the timeout is an error, so a replacement aborts with a null reason.
        // ctx.request.abort is an htmx 4.0.0 internal, so check it again when the htmx version changes.
        const abort = event.detail.ctx.request.abort;
        event.detail.ctx.request.abort = () => abort(request.timedOut() ? undefined : null);
        latestRequests.set(event.target, request);
        markBusy(event.target);
        if (event.target.dataset.pageLoad !== undefined) {
            setPageStatus('Loading…');
        }
        if (region && region !== event.target) {
            markBusy(region);
        }
        if (region) {
            request.values = new Map(fieldsIn(region).map((field) => [fieldKey(field), fieldState(field)]));
            regionState(region).active.add(request);
        }
    });

    // htmx 4.0.0 aborts a request that hx-sync replaces only while it is the current one, so a third request
    // leaves the second running and its answer can land after the third's.
    document.addEventListener('htmx:before:swap', (event) => {
        const request = requests.get(event.detail.ctx);
        if (request && latestRequests.get(request.element) !== request && request.element.getAttribute('hx-sync')?.endsWith(':replace')) {
            event.preventDefault();
            return;
        }
        if (!request?.region) {
            return;
        }
        for (const field of fieldsIn(request.region)) {
            const key = fieldKey(field);
            if (request.values.get(key) === fieldState(field)) {
                continue;
            }
            request.typed.set(key, { state: fieldState(field), form: field.form?.id ?? '', element: field });
            if (field === document.activeElement) {
                let start = null;
                let end = null;
                try {
                    ({ selectionStart: start, selectionEnd: end } = field);
                } catch {
                    // the field has no caret
                }
                request.active = { key, start, end };
            }
        }
    });

    const restoreTyped = (request) => {
        const replaced = [...request.typed.values()].filter((entry) => !entry.element.isConnected);
        const blocked = new Set(replaced.filter((entry) => entry.element.tagName === 'SELECT').map((entry) => entry.form));
        const fields = fieldsIn(request.region);
        let restored = false;
        for (const [key, entry] of request.typed) {
            if (entry.element.isConnected) {
                continue;
            }
            if (blocked.has(entry.form)) {
                request.dropped = true;
                continue;
            }
            const field = fields.find((candidate) => fieldKey(candidate) === key);
            if (!field) {
                continue;
            }
            if (fieldState(field) !== entry.state) {
                restored = true;
                setFieldState(field, entry.state);
            }
            if (request.active?.key === key) {
                field.focus({ preventScroll: true });
                try {
                    field.setSelectionRange(request.active.start, request.active.end);
                } catch {
                    // the field has no caret
                }
            }
        }
        return restored;
    };

    const noteUnsaved = (region) => {
        if (!region.dataset.status) {
            return;
        }
        const status = document.getElementById(region.dataset.status);
        if (!status) {
            return;
        }
        const note = 'Changes made while saving are not saved yet.';
        status.textContent = status.textContent ? `${status.textContent}. ${note}` : note;
    };

    document.addEventListener('htmx:finally:request', (event) => {
        const request = requests.get(event.detail.ctx);
        if (!request) {
            return;
        }
        clearBusy(request.element);
        if (request.element.dataset.pageLoad !== undefined) {
            // ctx.response and ctx.status are htmx 4.0.0 internals, so check them again when the htmx version changes.
            const { ctx } = event.detail;
            if (ctx.response?.status < 400 && !ctx.status.startsWith('error')) {
                setPageStatus('Loaded');
            } else {
                request.element.querySelector('p')?.remove();
                setPageStatus('Failed to load');
            }
        }
        const region = request.region;
        if (!region) {
            return;
        }
        const state = regionState(region);
        state.active.delete(request);
        if (region.isConnected && restoreTyped(request)) {
            noteUnsaved(region);
        }
        state.dropped ||= request.dropped;
        if (region !== request.element) {
            clearBusy(region);
        }
        if (state.active.size > 0) {
            return;
        }
        if (state.dropped && !document.getElementById('page-error')?.textContent) {
            showBanner('The change made while saving was not sent. Make it again.');
        }
        state.dropped = false;
    });

    document.addEventListener('input', (event) => {
        const field = event.target;
        const statusID = field.closest?.('[data-status]')?.dataset.status;
        if (statusID) {
            const status = document.getElementById(statusID);
            if (status) {
                status.textContent = '';
            }
        }
        if (field.getAttribute?.('aria-invalid') !== 'true') {
            return;
        }
        field.removeAttribute('aria-invalid');
        document.getElementById(`${field.id}-error`)?.remove();
        const remaining = (field.getAttribute('aria-describedby') ?? '').split(' ').filter((id) => id && id !== `${field.id}-error`);
        if (remaining.length > 0) {
            field.setAttribute('aria-describedby', remaining.join(' '));
        } else {
            field.removeAttribute('aria-describedby');
        }
    });

    const busyRegionOf = (target) => target.closest?.('[data-busy-region][aria-busy="true"]');

    // A second action that repeats the first (the same button over the same field values) loses nothing when it is
    // dropped, so only an action with a changed field or another button asks the person to make it again.
    const carriesNewAction = (region, button) => {
        const [first] = regionState(region).active;
        return !first || button !== first.trigger || fieldsIn(region).some((field) => first.values.get(fieldKey(field)) !== fieldState(field));
    };

    const reportDropped = (event, changed = false) => {
        const region = busyRegionOf(event.target);
        if (region) {
            const button = event.type === 'submit' ? event.submitter : event.target.closest?.('button');
            regionState(region).dropped ||= changed || carriesNewAction(region, button);
            showBanner('The last change is still being saved. Try again in a moment.');
        }
    };
    document.addEventListener('submit', (event) => {
        reportDropped(event);
        noteTrigger(event.submitter);
    }, true);
    document.addEventListener('click', (event) => {
        const button = event.target.closest?.('button');
        if (button) {
            reportDropped(event);
            noteTrigger(button);
        }
    }, true);

    document.addEventListener('change', (event) => {
        const field = event.target;
        if (!field.name || !(isChoice(field) || field.tagName === 'SELECT')) {
            return;
        }
        const region = busyRegionOf(field);
        if (region?.dataset.busyRegion !== 'replaced') {
            return;
        }
        const [first] = regionState(region).active;
        if (first?.values.has(fieldKey(field))) {
            setFieldState(field, first.values.get(fieldKey(field)));
        }
        event.stopImmediatePropagation();
        reportDropped(event, true);
    }, true);

    /** htmx reports a failed, a timed-out and a replaced request alike as htmx:error, and only a request that ran out its timeout was lost. */
    document.addEventListener('htmx:error', (event) => {
        const { ctx, error } = event.detail;
        const failed = error instanceof TypeError || (error?.name === 'AbortError' && requests.get(ctx)?.timedOut());
        if (ctx && !ctx.response && failed) {
            showBanner('The server could not be reached. Try again in a moment.');
        }
    });
})();
