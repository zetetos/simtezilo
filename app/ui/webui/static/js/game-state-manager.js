// game-state-manager.js - Manages game state display

(function () {
    'use strict';

    const gameStateText = document.getElementById('gameStateText');

    const stateConfig = {
        'main_menu': {
            textKey: 'runmode.home.gamestate.mainmenu',
            fallback: 'Main Menu',
            colorClass: 'state-main-menu'
        },
        'race_menu': {
            textKey: 'runmode.home.gamestate.racemenu',
            fallback: 'Race Menu',
            colorClass: 'state-race-menu'
        },
        'on_circuit': {
            textKey: 'runmode.home.gamestate.oncircuit',
            fallback: 'Live',
            colorClass: 'state-on-circuit'
        },
        'replay': {
            textKey: 'runmode.home.gamestate.replay',
            fallback: 'Replay',
            colorClass: 'state-replay'
        },
        'paused': {
            textKey: 'runmode.home.gamestate.paused',
            fallback: 'Paused',
            colorClass: 'state-paused'
        },
        'unknown': {
            textKey: 'runmode.home.gamestate.unknown',
            fallback: 'Unknown',
            colorClass: 'state-unknown'
        }
    };

    function updateGameState(state) {
        const config = stateConfig[state] || stateConfig['unknown'];

        // Update text with translation or fallback
        const translatedText = document.querySelector(`[data-i18n="${config.textKey}"]`)?.textContent;
        gameStateText.textContent = translatedText || config.fallback;
        gameStateText.className = `game-state-value ${config.colorClass}`;
    }

    // Listen for game state updates from vehicle WebSocket
    // (since game state is part of vehicle telemetry)
    window.addEventListener('gameStateUpdate', function (event) {
        updateGameState(event.detail.state);
    });

    // Initial state
    updateGameState('unknown');
})();
