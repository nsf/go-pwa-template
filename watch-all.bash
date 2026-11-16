#!/bin/bash

set -e

SESSION=myapp-watch
tmux new-session -s $SESSION -d 'tailwindcss --input static-src/style.css --output static/style.css --watch'
tmux split-window -v -t $SESSION 'templ generate -watch'
tmux split-window -v -t $SESSION 'air'
tmux attach -t $SESSION
