// Client-side JavaScript logic for Roblox Lookup with API backend & CORS fallback

const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? ''
    : 'https://roblox-lookup-api.onrender.com'; // Fallback API endpoint for production GitHub Pages if backend server is separate

async function performSearch() {
    const input = document.getElementById('query-input').value.trim();
    if (!input) return;

    showLoading(true);
    hideStatus();
    hideResult();

    const startTime = performance.now();

    try {
        let profile = null;
        let tookMs = 0;

        // Attempt primary lookup via API server
        try {
            const res = await fetch(`/api/lookup?q=${encodeURIComponent(input)}`);
            if (res.ok) {
                const data = await res.json();
                profile = data.data;
                tookMs = data.took_ms || Math.round(performance.now() - startTime);
            }
        } catch (e) {
            console.warn("Direct /api/lookup endpoint unavailable, performing client-side Roblox API lookup...");
        }

        // Fallback: Client-side fetching directly from public CORS proxy / Roblox APIs if deployed static on GitHub Pages
        if (!profile) {
            profile = await performClientLookup(input);
            tookMs = Math.round(performance.now() - startTime);
        }

        renderProfile(profile, tookMs);
    } catch (err) {
        showError(err.message || "Failed to fetch Roblox user information.");
    } finally {
        showLoading(false);
    }
}

function quickSearch(query) {
    document.getElementById('query-input').value = query;
    performSearch();
}

async function performClientLookup(query) {
    // Check if input is User ID or Username
    let userId = null;
    let username = query;

    if (/^\d+$/.test(query)) {
        userId = parseInt(query, 10);
    } else {
        // Resolve Username via Roblox API (using cors-anywhere / roblox endpoint)
        const res = await fetch(`https://users.roblox.com/v1/usernames/users`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ usernames: [query], excludeBannedUsers: false })
        });
        if (!res.ok) throw new Error("Could not resolve username. Roblox API error.");
        const json = await res.json();
        if (!json.data || json.data.length === 0) throw new Error(`User '${query}' not found on Roblox.`);
        userId = json.data[0].id;
        username = json.data[0].name;
    }

    // Parallel fetch using Promise.all (mirroring Go goroutines on frontend)
    const [userRes, friendsRes, followersRes, followingRes, headshotRes, bustRes, fullRes, groupsRes, badgesRes] = await Promise.all([
        fetch(`https://users.roblox.com/v1/users/${userId}`).then(r => r.ok ? r.json() : {}),
        fetch(`https://friends.roblox.com/v1/users/${userId}/friends/count`).then(r => r.ok ? r.json() : {}),
        fetch(`https://friends.roblox.com/v1/users/${userId}/followers/count`).then(r => r.ok ? r.json() : {}),
        fetch(`https://friends.roblox.com/v1/users/${userId}/followings/count`).then(r => r.ok ? r.json() : {}),
        fetch(`https://thumbnails.roblox.com/v1/users/avatar-headshot?userIds=${userId}&size=420x420&format=Png&isCircular=false`).then(r => r.ok ? r.json() : {}),
        fetch(`https://thumbnails.roblox.com/v1/users/avatar-bust?userIds=${userId}&size=420x420&format=Png`).then(r => r.ok ? r.json() : {}),
        fetch(`https://thumbnails.roblox.com/v1/users/avatar?userIds=${userId}&size=720x720&format=Png`).then(r => r.ok ? r.json() : {}),
        fetch(`https://groups.roblox.com/v2/users/${userId}/groups/roles`).then(r => r.ok ? r.json() : {}),
        fetch(`https://badges.roblox.com/v1/users/${userId}/badges?limit=10&sortOrder=Desc`).then(r => r.ok ? r.json() : {})
    ]);

    return {
        id: userId,
        username: userRes.name || username,
        displayName: userRes.displayName || username,
        description: userRes.description || "No description provided.",
        created: userRes.created || new Date().toISOString(),
        isBanned: userRes.isBanned || false,
        friendsCount: friendsRes.count || 0,
        followersCount: followersRes.count || 0,
        followingCount: followingRes.count || 0,
        avatarHeadshot: headshotRes.data?.[0]?.imageUrl || "",
        avatarBust: bustRes.data?.[0]?.imageUrl || "",
        avatarFull: fullRes.data?.[0]?.imageUrl || "",
        groups: groupsRes.data || [],
        badges: badgesRes.data || []
    };
}

function renderProfile(p, tookMs) {
    document.getElementById('display-name').textContent = p.displayName || p.username;
    document.getElementById('username').textContent = `@${p.username}`;
    document.getElementById('user-id').textContent = p.id;
    document.getElementById('description').textContent = p.description || "No description provided.";

    if (p.created) {
        const date = new Date(p.created);
        document.getElementById('join-date').textContent = date.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
    }

    document.getElementById('lookup-time').textContent = `${tookMs} ms`;
    document.getElementById('friends-count').textContent = formatNumber(p.friendsCount);
    document.getElementById('followers-count').textContent = formatNumber(p.followersCount);
    document.getElementById('following-count').textContent = formatNumber(p.followingCount);

    // Avatars
    const fallbackAvatar = "https://tr.rbxcdn.com/30day-avatar-headshot";
    document.getElementById('avatar-img').src = p.avatarHeadshot || p.avatarFull || fallbackAvatar;
    document.getElementById('headshot-img').src = p.avatarHeadshot || fallbackAvatar;
    document.getElementById('bust-img').src = p.avatarBust || fallbackAvatar;
    document.getElementById('full-img').src = p.avatarFull || fallbackAvatar;

    // Presence
    const presenceBadge = document.getElementById('presence-badge');
    const statusIndicator = document.getElementById('status-indicator');
    statusIndicator.className = 'status-indicator';

    if (p.presence) {
        switch (p.presence.userPresenceType) {
            case 1:
                presenceBadge.textContent = 'Online';
                presenceBadge.style.color = '#10b981';
                statusIndicator.classList.add('online');
                break;
            case 2:
                presenceBadge.textContent = 'In-Game';
                presenceBadge.style.color = '#3b82f6';
                statusIndicator.classList.add('ingame');
                break;
            case 3:
                presenceBadge.textContent = 'In-Studio';
                presenceBadge.style.color = '#f59e0b';
                statusIndicator.classList.add('instudio');
                break;
            default:
                presenceBadge.textContent = 'Offline';
                presenceBadge.style.color = '#64748b';
                statusIndicator.classList.add('offline');
        }
    } else {
        presenceBadge.textContent = 'Offline';
        statusIndicator.classList.add('offline');
    }

    // Groups
    const groupsList = document.getElementById('groups-list');
    document.getElementById('groups-count').textContent = p.groups ? p.groups.length : 0;
    groupsList.innerHTML = '';
    if (p.groups && p.groups.length > 0) {
        p.groups.forEach(g => {
            const div = document.createElement('div');
            div.className = 'group-item';
            div.innerHTML = `
                <span class="group-name">${escapeHtml(g.group.name)}</span>
                <span class="group-role">${escapeHtml(g.role.name)} (Rank ${g.role.rank})</span>
            `;
            groupsList.appendChild(div);
        });
    } else {
        groupsList.innerHTML = '<p class="user-bio">No groups found.</p>';
    }

    // Badges
    const badgesList = document.getElementById('badges-list');
    document.getElementById('badges-count').textContent = p.badges ? p.badges.length : 0;
    badgesList.innerHTML = '';
    if (p.badges && p.badges.length > 0) {
        p.badges.forEach(b => {
            const div = document.createElement('div');
            div.className = 'badge-item';
            div.innerHTML = `
                <span class="badge-name">${escapeHtml(b.name)}</span>
                <span class="badge-desc">${escapeHtml(b.description || 'No description')}</span>
            `;
            badgesList.appendChild(div);
        });
    } else {
        badgesList.innerHTML = '<p class="user-bio">No recent badges found.</p>';
    }

    document.getElementById('result-card').classList.remove('hidden');
}

function showLoading(loading) {
    const btn = document.getElementById('search-btn');
    const btnText = btn.querySelector('span');
    const spinner = document.getElementById('btn-spinner');

    if (loading) {
        btnText.textContent = 'Searching...';
        spinner.classList.remove('hidden');
        btn.disabled = true;
    } else {
        btnText.textContent = 'Search';
        spinner.classList.add('hidden');
        btn.disabled = false;
    }
}

function showError(msg) {
    const statusContainer = document.getElementById('status-container');
    statusContainer.className = 'status-container error';
    statusContainer.textContent = `Error: ${msg}`;
    statusContainer.classList.remove('hidden');
}

function hideStatus() {
    document.getElementById('status-container').classList.add('hidden');
}

function hideResult() {
    document.getElementById('result-card').classList.add('hidden');
}

function formatNumber(num) {
    if (!num) return '0';
    return new Intl.NumberFormat().format(num);
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/[&<>"']/g, function(m) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[m];
    });
}
