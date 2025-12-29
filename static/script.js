let targetSentence = "The quick brown fox jumps over the lazy dog.";

let currentIndex = 0;
let mistakeCount = 0;
const MAX_MISTAKES = 3;

let sentenceContainer;
let winMessage;
let mainContainer;
let mistakeCounter;
let playersProgressLines;
let startGameBtn;
let instructionsText;

// WebSocket and multiplayer state
let ws = null;
let playerID = null;
let channelID = null;
let username = null;
let gameStatus = "waiting"; // "waiting", "playing", "finished"
let allPlayers = {}; // Map of playerID -> player data
let playerColors = {}; // Map of playerID -> color

const PLAYER_COLORS = [
    '#7289DA', '#43B581', '#F04747', '#FAA61A', '#9C84EF',
    '#EC4444', '#FF73FA', '#00D9FF', '#FFB800', '#00FF88'
];

function getPlayerColor(playerID) {
    if (!playerColors[playerID]) {
        const colorIndex = Object.keys(playerColors).length % PLAYER_COLORS.length;
        playerColors[playerID] = PLAYER_COLORS[colorIndex];
    }
    return playerColors[playerID];
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        console.log('WebSocket connected');
        const urlParams = new URLSearchParams(window.location.search);
        channelID = urlParams.get('channel') || 'default';
        
        username = urlParams.get('username') || 'Player' + Math.floor(Math.random() * 1000);
        
        joinRoom(channelID, username);
    };
    
    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);
        handleWebSocketMessage(message);
    };
    
    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
        instructionsText.textContent = 'Connection error. Please refresh.';
    };
    
    ws.onclose = function() {
        console.log('WebSocket disconnected');
        instructionsText.textContent = 'Disconnected. Please refresh.';
        // Try to reconnect after 3 seconds
        setTimeout(connectWebSocket, 3000);
    };
}

function joinRoom(channelID, username) {
    const message = {
        type: 'JoinRoom',
        channelID: channelID,
        username: username
    };
    ws.send(JSON.stringify(message));
}

function startGame() {
    if (ws && ws.readyState === WebSocket.OPEN && gameStatus === 'waiting') {
        const message = {
            type: 'StartGame'
        };
        ws.send(JSON.stringify(message));
        startGameBtn.style.display = 'none';
        instructionsText.textContent = 'Game started! Start typing to begin. Match each character exactly!';
        mainContainer.focus();
    }
}

function sendProgressUpdate(index, mistakes) {
    if (ws && ws.readyState === WebSocket.OPEN && gameStatus === 'playing') {
        const message = {
            type: 'ProgressUpdate',
            currentIndex: index,
            mistakeCount: mistakes
        };
        ws.send(JSON.stringify(message));
    }
}

function handleWebSocketMessage(message) {
    switch (message.type) {
        case 'Joined':
            playerID = message.playerID;
            targetSentence = message.targetSentence || targetSentence;
            allPlayers = {};
            if (message.players) {
                message.players.forEach(player => {
                    allPlayers[player.id] = player;
                });
            }
            updatePlayersProgress(message.players || []);
            initializeSentence();
            if (gameStatus === 'waiting') {
                startGameBtn.style.display = 'inline-block';
                instructionsText.textContent = 'Click "Start Game" to begin. Other players can join!';
            }
            break;
            
        case 'PlayerJoined':
            if (message.players) {
                message.players.forEach(player => {
                    allPlayers[player.id] = player;
                });
                updatePlayersProgress(message.players);
            }
            break;
            
        case 'GameStarted':
            gameStatus = 'playing';
            targetSentence = message.targetSentence || targetSentence;
            initializeSentence();
            startGameBtn.style.display = 'none';
            instructionsText.textContent = 'Game started! Start typing to begin. Match each character exactly!';
            mainContainer.focus();
            break;
            
        case 'PlayerProgress':
            if (message.players) {
                message.players.forEach(player => {
                    allPlayers[player.id] = player;
                });
                updatePlayersProgress(message.players);
            }
            break;
            
        case 'GameFinished':
            gameStatus = 'finished';
            const winnerText = message.winner || 'Someone';
            winMessage.innerHTML = `<h2>Game Finished!</h2><p>${winnerText} completed the challenge first!</p>`;
            winMessage.style.display = 'block';
            sentenceContainer.style.opacity = '0.7';
            instructionsText.textContent = 'Game finished! Refresh to play again.';
            break;
    }
}

function updatePlayersProgress(players) {
    if (!playersProgressLines) return;
    
    playersProgressLines.innerHTML = '';
    
    players.forEach(player => {
        const line = renderPlayerLine(player);
        playersProgressLines.appendChild(line);
    });
}

function renderPlayerLine(player) {
    const line = document.createElement('div');
    line.className = 'player-progress-line';
    line.setAttribute('data-player-id', player.id);
    
    const color = getPlayerColor(player.id);
    const isCurrentPlayer = player.id === playerID;
    
    const progressPercent = targetSentence.length > 0 
        ? (player.currentIndex / targetSentence.length) * 100 
        : 0;
    
    line.innerHTML = `
        <div class="player-info">
            <span class="player-marker" style="background-color: ${color}"></span>
            <span class="player-name" ${isCurrentPlayer ? 'style="font-weight: bold;"' : ''}>${player.username}</span>
            <span class="player-stats">${player.currentIndex}/${targetSentence.length}</span>
        </div>
        <div class="player-progress-bar">
            <div class="player-progress-fill" style="width: ${progressPercent}%; background-color: ${color}"></div>
        </div>
    `;
    
    return line;
}

function initializeSentence() {
    if (!sentenceContainer) return;
    
    sentenceContainer.innerHTML = '';
    
    for (let i = 0; i < targetSentence.length; i++) {
        const span = document.createElement('span');
        span.textContent = targetSentence[i];
        span.setAttribute('data-index', i);
        
        if (targetSentence[i] === ' ') {
            span.classList.add('space-char');
        }
        
        if (i === 0) {
            span.classList.add('current');
        }
        
        sentenceContainer.appendChild(span);
    }
    
    // Add player position markers
    updatePlayerMarkers();
}

function updatePlayerMarkers() {
    if (!sentenceContainer) return;
    
    sentenceContainer.querySelectorAll('.player-marker-position').forEach(marker => marker.remove());
    
    // Add markers for all players except current player
    Object.values(allPlayers).forEach(player => {
        if (player.id !== playerID && player.currentIndex < targetSentence.length) {
            const span = getSpanAtIndex(player.currentIndex);
            if (span) {
                const marker = document.createElement('span');
                marker.className = 'player-marker-position';
                marker.style.backgroundColor = getPlayerColor(player.id);
                marker.setAttribute('data-player-id', player.id);
                marker.title = player.username;
                span.appendChild(marker);
            }
        }
    });
}

function getSpanAtIndex(index) {
    return sentenceContainer.querySelector(`[data-index="${index}"]`);
}

function updateMistakeCounter() {
    if (mistakeCounter) {
        mistakeCounter.textContent = `Mistakes: ${mistakeCount}/${MAX_MISTAKES}`;
    }
}

function resetProgress() {
    currentIndex = 0;
    mistakeCount = 0;
    updateMistakeCounter();
    
    const allSpans = sentenceContainer.querySelectorAll('span');
    allSpans.forEach((span, index) => {
        span.classList.remove('correct', 'current', 'incorrect');
        if (index === 0) {
            span.classList.add('current');
        }
    });
    
    document.body.classList.add('critical-error');
    setTimeout(() => {
        document.body.classList.remove('critical-error');
    }, 600);
    
    sendProgressUpdate(currentIndex, mistakeCount);
}

window.addEventListener('DOMContentLoaded', function() {
    sentenceContainer = document.getElementById('sentenceContainer');
    winMessage = document.getElementById('winMessage');
    mainContainer = document.getElementById('mainContainer');
    mistakeCounter = document.getElementById('mistakeCounter');
    playersProgressLines = document.getElementById('playersProgressLines');
    startGameBtn = document.getElementById('startGameBtn');
    instructionsText = document.getElementById('instructionsText');
    
    updateMistakeCounter();
    
    mainContainer.focus();
    
    mainContainer.addEventListener('click', function() {
        mainContainer.focus();
    });
    
    if (startGameBtn) {
        startGameBtn.addEventListener('click', startGame);
    }
    
    connectWebSocket();
});

function handleKeydown(event) {
    if (!sentenceContainer || gameStatus !== 'playing') {
        return;
    }
    
    if (event.key === 'Shift' || event.key === 'Control' || event.key === 'Alt' || event.key === 'Meta' || event.key === 'CapsLock') {
        return;
    }
    
    if (event.key.length === 1 || event.key === 'Backspace' || event.key === 'Enter') {
        event.preventDefault();
    }
    
    if (event.key === 'Backspace') {
        if (currentIndex > 0) {
            const currentSpan = getSpanAtIndex(currentIndex);
            if (currentSpan) {
                currentSpan.classList.remove('current');
            }
            
            currentIndex--;
            
            const prevSpan = getSpanAtIndex(currentIndex);
            if (prevSpan) {
                prevSpan.classList.remove('correct');
                prevSpan.classList.add('current');
            }
            
            mistakeCount = 0;
            updateMistakeCounter();
            sendProgressUpdate(currentIndex, mistakeCount);
            updatePlayerMarkers();
        }
        return;
    }
    
    if (event.key === 'Enter') {
        return;
    }
    
    if (currentIndex >= targetSentence.length) {
        return;
    }
    
    const expectedChar = targetSentence[currentIndex].toLowerCase();
    const pressedKey = event.key.toLowerCase();
    
    const currentSpan = getSpanAtIndex(currentIndex);
    
    if (pressedKey === expectedChar) {
        currentSpan.classList.remove('current');
        currentSpan.classList.add('correct');
        
        mistakeCount = 0;
        updateMistakeCounter();
        
        currentIndex++;
        
        if (currentIndex >= targetSentence.length) {
            // Game finished
            if (ws && ws.readyState === WebSocket.OPEN) {
                const message = {
                    type: 'GameFinished'
                };
                ws.send(JSON.stringify(message));
            }
            winMessage.innerHTML = '<h2>Finished!</h2><p>Great job! You completed the typing challenge.</p>';
            winMessage.style.display = 'block';
            sentenceContainer.style.opacity = '0.7';
            return;
        }
        
        const nextSpan = getSpanAtIndex(currentIndex);
        if (nextSpan) {
            nextSpan.classList.add('current');
        }
        
        sendProgressUpdate(currentIndex, mistakeCount);
        updatePlayerMarkers();
    } else {
        mistakeCount++;
        updateMistakeCounter();
        
        currentSpan.classList.add('incorrect');
        sentenceContainer.classList.add('shake');
        
        setTimeout(() => {
            currentSpan.classList.remove('incorrect');
            sentenceContainer.classList.remove('shake');
        }, 500);
        
        sendProgressUpdate(currentIndex, mistakeCount);
        
        if (mistakeCount >= MAX_MISTAKES) {
            setTimeout(() => {
                resetProgress();
            }, 500);
        }
    }
}

window.addEventListener('keydown', handleKeydown);
