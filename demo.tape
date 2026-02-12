Output assets/demo.gif
Set FontSize 16
Set Width 900
Set Height 550
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Margin 10
Set MarginFill "#1e1e2e"
Set BorderRadius 8

Sleep 500ms

Type "claude-rig create fullstack --link-auth"
Enter
Sleep 2s

Type "claude-rig set-args fullstack -- --dangerously-skip-permissions"
Enter
Sleep 2s

Type "claude-rig list"
Enter
Sleep 3s

Type "claude-rig doctor"
Enter
Sleep 4s

Type "claude-rig launch fullstack"
Sleep 1s
