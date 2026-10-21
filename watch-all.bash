#!/bin/bash

set -e

SESSION=myapp-watch
tmux new -s $SESSION -d
tmux splitw -v -t $SESSION
tmux splitw -v -t $SESSION
tmux send-keys -t $SESSION:.0 'tailwindcss --input style/style.css --output static/style.css --watch' Enter
tmux send-keys -t $SESSION:.1 'templ generate -watch' Enter
tmux send-keys -t $SESSION:.2 'air' Enter
tmux a -t $SESSION
