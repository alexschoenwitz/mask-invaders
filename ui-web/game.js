// Game State
class Game {
    constructor(apiURL, playerName, playerID, playerToken) {
        this.apiURL = apiURL;
        this.playerName = playerName;
        this.playerID = playerID;
        this.playerToken = playerToken;
        
        this.canvas = document.getElementById('game-canvas');
        this.ctx = this.canvas.getContext('2d');
        this.width = 800;
        this.height = 800;
        
        this.state = null;
        this.cities = new Map();
        this.movements = [];
        this.playerColors = new Map();
        this.colorPalette = [
            '#ff0000', '#00ff00', '#0000ff', '#ffff00',
            '#ff00ff', '#00ffff', '#ff8000', '#8000ff',
            '#ffc0cb', '#008000', '#808080', '#ffffff'
        ];
        
        // UI State
        this.uiState = 'idle'; // idle, citySelected, attackTarget, produceType
        this.selectedCity = null;
        this.hoveredCity = null;
        this.errorMessage = '';
        this.successMessage = '';
        this.messageTime = 0;
        
        this.currentTurn = 0;
        this.gameStarted = false;
        
        // Initialize
        this.setupEventListeners();
        this.startPolling();
        this.render();
    }
    
    setupEventListeners() {
        this.canvas.addEventListener('click', (e) => this.handleClick(e));
        this.canvas.addEventListener('mousemove', (e) => this.handleMouseMove(e));
        document.addEventListener('keydown', (e) => this.handleKeyPress(e));
    }
    
    async startPolling() {
        // Initial fetch
        await this.fetchState();
        
        // Poll every 100ms
        setInterval(() => this.fetchState(), 100);
    }
    
    async fetchState() {
        try {
            const response = await fetch(`${this.apiURL}/v1/state`);
            if (!response.ok) return;
            
            const data = await response.json();
            if (data.state) {
                this.updateState(data.state);
            }
        } catch (error) {
            console.error('Failed to fetch state:', error);
        }
    }
    
    updateState(state) {
        this.state = state;
        this.currentTurn = state.turn || 0;
        
        // Initialize cities on first state
        if (this.cities.size === 0 && state.cities) {
            this.initializeCities(state.cities);
        }
        
        // Update city states
        if (state.cities) {
            for (const [name, city] of Object.entries(state.cities)) {
                if (this.cities.has(name)) {
                    const cityDisplay = this.cities.get(name);
                    cityDisplay.player = city.player;
                    cityDisplay.troops = city.troops || {};
                }
            }
        }
        
        // Update movements
        this.movements = state.movements || [];
        
        if (!this.gameStarted && this.currentTurn > 0) {
            this.gameStarted = true;
        }
    }
    
    initializeCities(cities) {
        const cityNames = Object.keys(cities);
        const numCities = cityNames.length;
        const centerX = this.width / 2;
        const centerY = this.height / 2;
        const radius = Math.min(this.width, this.height) * 0.3;
        
        cityNames.forEach((name, i) => {
            let x, y;
            if (numCities === 1) {
                x = centerX;
                y = centerY;
            } else {
                const angle = (2 * Math.PI * i) / numCities;
                x = centerX + radius * Math.cos(angle);
                y = centerY + radius * Math.sin(angle);
            }
            
            this.cities.set(name, {
                name: name,
                player: cities[name].player,
                troops: cities[name].troops || {},
                x: x,
                y: y,
                size: 50
            });
        });
        
        // Assign player colors
        const players = new Set();
        for (const city of Object.values(cities)) {
            players.add(city.player);
        }
        let colorIdx = 0;
        for (const player of players) {
            this.playerColors.set(player, this.colorPalette[colorIdx % this.colorPalette.length]);
            colorIdx++;
        }
    }
    
    handleMouseMove(e) {
        const rect = this.canvas.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        
        this.hoveredCity = null;
        for (const city of this.cities.values()) {
            const dist = Math.sqrt((x - city.x) ** 2 + (y - city.y) ** 2);
            if (dist < 40) {
                this.hoveredCity = city;
                break;
            }
        }
    }
    
    handleClick(e) {
        const rect = this.canvas.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        
        console.log('Click:', this.uiState, this.hoveredCity);
        
        switch (this.uiState) {
            case 'idle':
                if (this.hoveredCity && this.hoveredCity.player === this.playerID) {
                    this.selectedCity = this.hoveredCity;
                    this.uiState = 'citySelected';
                } else if (this.hoveredCity) {
                    this.showError('That castle belongs to another player!');
                }
                break;
                
            case 'citySelected':
                // Check if clicking buttons
                if (this.checkButtonClick(x, y)) {
                    return;
                }
                // Clicking same city keeps it selected
                if (this.hoveredCity === this.selectedCity) {
                    return;
                }
                // Deselect
                this.selectedCity = null;
                this.uiState = 'idle';
                break;
                
            case 'attackTarget':
                if (this.hoveredCity && this.hoveredCity !== this.selectedCity) {
                    this.performAttack(this.hoveredCity);
                    this.selectedCity = null;
                    this.uiState = 'idle';
                } else {
                    this.uiState = 'citySelected';
                }
                break;
                
            case 'produceType':
                if (this.checkTroopTypeClick(x, y)) {
                    return;
                }
                this.uiState = 'citySelected';
                break;
        }
    }
    
    checkButtonClick(x, y) {
        if (!this.selectedCity) return false;
        
        const cityX = this.selectedCity.x;
        const cityY = this.selectedCity.y - 60;
        
        // Attack button
        if (x >= cityX - 80 && x <= cityX - 10 &&
            y >= cityY && y <= cityY + 30) {
            if (this.hasTroops(this.selectedCity)) {
                this.uiState = 'attackTarget';
            } else {
                this.showError('No troops to attack with!');
            }
            return true;
        }
        
        // Produce button
        if (x >= cityX + 10 && x <= cityX + 90 &&
            y >= cityY && y <= cityY + 30) {
            this.uiState = 'produceType';
            return true;
        }
        
        return false;
    }
    
    checkTroopTypeClick(x, y) {
        if (!this.selectedCity) return false;
        
        const cityX = this.selectedCity.x;
        const cityY = this.selectedCity.y;
        const buttonY = cityY + 40;
        const startX = cityX - 80;
        
        const troopTypes = ['A', 'B', 'C'];
        for (let i = 0; i < troopTypes.length; i++) {
            const buttonX = startX + i * 55;
            if (x >= buttonX && x <= buttonX + 50 &&
                y >= buttonY && y <= buttonY + 35) {
                this.performProduce(troopTypes[i]);
                this.selectedCity = null;
                this.uiState = 'idle';
                return true;
            }
        }
        
        return false;
    }
    
    handleKeyPress(e) {
        if (e.key === 's' || e.key === 'S') {
            this.startGame();
        } else if (e.key === 'Escape') {
            this.selectedCity = null;
            this.uiState = 'idle';
        }
    }
    
    hasTroops(city) {
        const troops = city.troops || {};
        return (troops.A || 0) > 0 || (troops.B || 0) > 0 || (troops.C || 0) > 0;
    }
    
    async startGame() {
        try {
            const response = await fetch(`${this.apiURL}/v1/start`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': this.playerToken
                },
                body: '{}'
            });
            
            if (response.ok) {
                this.showSuccess('Game started!');
            } else {
                this.showError('Failed to start game');
            }
        } catch (error) {
            this.showError('Failed to start game: ' + error.message);
        }
    }
    
    async performAttack(target) {
        const troops = this.selectedCity.troops || {};
        const action = {
            player: this.playerID,
            attack: {
                from: this.selectedCity.name,
                city: target.name,
                troops: {
                    A: troops.A || 0,
                    B: troops.B || 0,
                    C: troops.C || 0
                }
            }
        };
        
        await this.sendAction(action);
        this.showSuccess(`Attacking ${target.name}!`);
    }
    
    async performProduce(troopType) {
        const action = {
            player: this.playerID,
            createTroop: {
                in: this.selectedCity.name,
                type: troopType
            }
        };
        
        await this.sendAction(action);
        this.showSuccess(`Producing troop ${troopType}!`);
    }
    
    async sendAction(action) {
        try {
            const response = await fetch(`${this.apiURL}/v1/action`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': this.playerToken
                },
                body: JSON.stringify({ action })
            });
            
            if (!response.ok) {
                const text = await response.text();
                this.showError('Action failed: ' + text);
            }
        } catch (error) {
            this.showError('Action failed: ' + error.message);
        }
    }
    
    showError(msg) {
        this.errorMessage = msg;
        this.messageTime = Date.now();
        console.error(msg);
    }
    
    showSuccess(msg) {
        this.successMessage = msg;
        this.messageTime = Date.now();
        console.log(msg);
    }
    
    render() {
        // Clear canvas
        this.ctx.fillStyle = '#0f3460';
        this.ctx.fillRect(0, 0, this.width, this.height);
        
        // Draw cities
        for (const city of this.cities.values()) {
            this.drawCity(city);
        }
        
        // Draw movements
        for (const movement of this.movements) {
            this.drawMovement(movement);
        }
        
        // Draw UI overlays
        this.drawUI();
        
        // Continue rendering
        requestAnimationFrame(() => this.render());
    }
    
    drawCity(city) {
        const ctx = this.ctx;
        const color = this.playerColors.get(city.player) || '#888';
        
        // Highlight if hovered
        if (city === this.hoveredCity) {
            ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)';
            ctx.lineWidth = 4;
            ctx.beginPath();
            ctx.arc(city.x, city.y, 45, 0, Math.PI * 2);
            ctx.stroke();
        }
        
        // Highlight if selected
        if (city === this.selectedCity) {
            ctx.strokeStyle = 'rgba(255, 255, 0, 0.8)';
            ctx.lineWidth = 5;
            ctx.beginPath();
            ctx.arc(city.x, city.y, 48, 0, Math.PI * 2);
            ctx.stroke();
        }
        
        // Draw castle circle
        ctx.fillStyle = color;
        ctx.globalAlpha = 0.3;
        ctx.beginPath();
        ctx.arc(city.x, city.y, 35, 0, Math.PI * 2);
        ctx.fill();
        ctx.globalAlpha = 1.0;
        
        // Draw castle border
        ctx.strokeStyle = color;
        ctx.lineWidth = 3;
        ctx.beginPath();
        ctx.arc(city.x, city.y, 35, 0, Math.PI * 2);
        ctx.stroke();
        
        // Draw castle name
        ctx.fillStyle = '#fff';
        ctx.font = '12px Arial';
        ctx.textAlign = 'center';
        const shortName = city.name.substring(0, 10);
        ctx.fillText(shortName, city.x, city.y - 45);
        
        // Draw troop counts
        const troops = city.troops || {};
        ctx.font = '11px Arial';
        ctx.textAlign = 'left';
        let yOffset = -20;
        for (const type of ['A', 'B', 'C']) {
            const count = troops[type] || 0;
            ctx.fillText(`${type}:${count}`, city.x + 30, city.y + yOffset);
            yOffset += 14;
        }
    }
    
    drawMovement(movement) {
        const fromCity = this.cities.get(movement.from);
        if (!fromCity) return;
        
        let toX, toY;
        if (movement.city) {
            const toCity = this.cities.get(movement.city);
            if (!toCity) return;
            toX = toCity.x;
            toY = toCity.y;
        } else {
            return; // Skip mine movements for now
        }
        
        const color = this.playerColors.get(movement.player) || '#888';
        
        // Calculate progress (simplified - just show at midpoint)
        const x = (fromCity.x + toX) / 2;
        const y = (fromCity.y + toY) / 2;
        
        // Draw arrow
        this.ctx.strokeStyle = color;
        this.ctx.lineWidth = 2;
        this.ctx.setLineDash([5, 5]);
        this.ctx.beginPath();
        this.ctx.moveTo(fromCity.x, fromCity.y);
        this.ctx.lineTo(toX, toY);
        this.ctx.stroke();
        this.ctx.setLineDash([]);
        
        // Draw troop indicator
        this.ctx.fillStyle = color;
        this.ctx.beginPath();
        this.ctx.arc(x, y, 8, 0, Math.PI * 2);
        this.ctx.fill();
        
        // Draw troop count
        const troops = movement.troops || {};
        const total = (troops.A || 0) + (troops.B || 0) + (troops.C || 0);
        this.ctx.fillStyle = '#fff';
        this.ctx.font = '10px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.fillText(total.toString(), x, y + 20);
    }
    
    drawUI() {
        const ctx = this.ctx;
        
        // Draw action buttons if city selected
        if (this.selectedCity && this.uiState === 'citySelected') {
            const cityX = this.selectedCity.x;
            const cityY = this.selectedCity.y - 60;
            
            // Attack button
            this.drawButton(cityX - 80, cityY, 70, 30, 'ATTACK', '#c83232');
            
            // Produce button
            this.drawButton(cityX + 10, cityY, 80, 30, 'PRODUCE', '#32c832');
            
            ctx.fillStyle = '#fff';
            ctx.font = '12px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Choose action:', cityX, cityY - 5);
        }
        
        // Draw troop type selection
        if (this.selectedCity && this.uiState === 'produceType') {
            const cityX = this.selectedCity.x;
            const cityY = this.selectedCity.y + 40;
            const startX = cityX - 80;
            
            ctx.fillStyle = '#fff';
            ctx.font = '12px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Select troop type:', cityX, cityY - 10);
            
            const types = ['A', 'B', 'C'];
            const names = ['Archer', 'Knight', 'Infantry'];
            for (let i = 0; i < types.length; i++) {
                const x = startX + i * 55;
                this.drawButton(x, cityY, 50, 35, types[i], '#6464c8');
                ctx.font = '10px Arial';
                ctx.fillText(names[i], x + 25, cityY + 50);
            }
        }
        
        // Draw attack target instruction
        if (this.uiState === 'attackTarget') {
            ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
            ctx.fillRect(200, 350, 400, 30);
            ctx.fillStyle = '#fff';
            ctx.font = '14px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Click target castle (ESC to cancel)', 400, 370);
        }
        
        // Update status overlay
        this.updateStatusOverlay();
    }
    
    drawButton(x, y, w, h, text, color) {
        const ctx = this.ctx;
        
        // Button background
        ctx.fillStyle = color;
        ctx.fillRect(x, y, w, h);
        
        // Button border
        ctx.strokeStyle = '#fff';
        ctx.lineWidth = 2;
        ctx.strokeRect(x, y, w, h);
        
        // Button text
        ctx.fillStyle = '#fff';
        ctx.font = 'bold 12px Arial';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(text, x + w/2, y + h/2);
        ctx.textBaseline = 'alphabetic';
    }
    
    updateStatusOverlay() {
        const statusDiv = document.getElementById('status');
        const messagesDiv = document.getElementById('messages');
        
        // Status text
        let status = `Player: ${this.playerName} | Turn: ${this.currentTurn} | `;
        status += `Press 'S' to start | ESC to cancel`;
        statusDiv.textContent = status;
        
        // Messages
        const now = Date.now();
        if (now - this.messageTime < 3000) {
            let msg = '';
            if (this.errorMessage) {
                msg = `<span class="error">ERROR: ${this.errorMessage}</span>`;
            } else if (this.successMessage) {
                msg = `<span class="success">SUCCESS: ${this.successMessage}</span>`;
            }
            messagesDiv.innerHTML = msg;
        } else {
            messagesDiv.innerHTML = '';
            this.errorMessage = '';
            this.successMessage = '';
        }
    }
}

// Login handler
document.getElementById('join-btn').addEventListener('click', async () => {
    const playerName = document.getElementById('player-name').value.trim();
    const apiURL = document.getElementById('api-url').value.trim();
    const messageDiv = document.getElementById('login-message');
    const button = document.getElementById('join-btn');
    
    if (!playerName) {
        messageDiv.innerHTML = '<span class="error">Please enter a player name</span>';
        return;
    }
    
    button.disabled = true;
    messageDiv.textContent = 'Connecting...';
    
    try {
        // Register player
        const response = await fetch(`${apiURL}/v1/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: playerName })
        });
        
        if (!response.ok) {
            throw new Error('Registration failed');
        }
        
        const data = await response.json();
        console.log('Registered:', data);
        
        // Hide login, show game
        document.getElementById('login-panel').classList.add('hidden');
        document.getElementById('game-container').classList.remove('hidden');
        
        // Start game
        window.game = new Game(apiURL, playerName, data.id, data.token);
        
    } catch (error) {
        messageDiv.innerHTML = `<span class="error">Failed to connect: ${error.message}</span>`;
        button.disabled = false;
    }
});

// Allow enter key in inputs
document.getElementById('player-name').addEventListener('keypress', (e) => {
    if (e.key === 'Enter') document.getElementById('join-btn').click();
});
