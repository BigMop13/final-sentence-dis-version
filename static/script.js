const targetSentence = "The quick brown fox jumps over the lazy dog.";

let currentIndex = 0;
let mistakeCount = 0;
const MAX_MISTAKES = 3;

let sentenceContainer;
let winMessage;
let mainContainer;
let mistakeCounter;

function initializeSentence() {
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
}

function getSpanAtIndex(index) {
    return sentenceContainer.querySelector(`[data-index="${index}"]`);
}

function updateMistakeCounter() {
    mistakeCounter.textContent = `Mistakes: ${mistakeCount}/${MAX_MISTAKES}`;
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
}

window.addEventListener('DOMContentLoaded', function() {
    sentenceContainer = document.getElementById('sentenceContainer');
    winMessage = document.getElementById('winMessage');
    mainContainer = document.getElementById('mainContainer');
    mistakeCounter = document.getElementById('mistakeCounter');
    
    initializeSentence();
    updateMistakeCounter();
    
    mainContainer.focus();
    
    mainContainer.addEventListener('click', function() {
        mainContainer.focus();
    });
});

function handleKeydown(event) {
    if (!sentenceContainer) {
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
            winMessage.style.display = 'block';
            sentenceContainer.style.opacity = '0.7';
            return;
        }
        
        const nextSpan = getSpanAtIndex(currentIndex);
        if (nextSpan) {
            nextSpan.classList.add('current');
        }
    } else {
        mistakeCount++;
        updateMistakeCounter();
        
        currentSpan.classList.add('incorrect');
        sentenceContainer.classList.add('shake');
        
        setTimeout(() => {
            currentSpan.classList.remove('incorrect');
            sentenceContainer.classList.remove('shake');
        }, 500);
        
        if (mistakeCount >= MAX_MISTAKES) {
            setTimeout(() => {
                resetProgress();
            }, 500);
        }
    }
}

window.addEventListener('keydown', handleKeydown);
