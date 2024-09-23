Output demo.gif

Set Width 1600
Set Height 850
Set WindowBar Colorful
Set FontSize 14
Set PlaybackSpeed 0.75

Set Shell fish

# Show off UI.
Sleep 500ms
Type "./labcoat -config labcoat-config.toml"
Enter
Sleep 5s
Type "j"
Sleep 3s
Type "j"
Sleep 3s
Type "j"
Sleep 3s
Type "/"
Sleep 1s
Type "lab"
Sleep 500ms
Enter
Sleep 1.5s
Type "fd"
Sleep 3s
Type "!"
Sleep 500ms
Type "systemctl status "
Sleep 500ms
Type "postgresql"
Sleep 1.5s
Enter
Sleep 3.5s
Type "q"
Sleep 2s

# Add PostgreSQL to db-lab config.
Type "vim flake.nix"
Enter
Sleep 3s
Type "/d"
Sleep 500ms
Type "b-lab"
Sleep 1.5s
Enter
Sleep 500ms
Type "zz"
Sleep 1.5s
Type "jjo"
Sleep 500ms
Type "{ services.postgresql.enable"
Sleep 200ms
Type " = true; }"
Sleep 500ms
Escape
Type ":wq"
Sleep 500ms
Enter
Sleep 2s

# Deploy db-lab.
Up
Up
Sleep 500ms
Enter
Sleep 3s
Type "d"
Sleep 10.5s
Type "s"
Sleep 2.5s
Type "!systemctl status "
Sleep 200ms
Type "postgresql"
Enter
Sleep 3s

# SSH demo.
Type "i"
Sleep 2s
Type "# <<< on the db-lab host; one touch SSH!"
Enter
Sleep 1s
Type "su - postgres"
Enter
Sleep 500ms
Type "psql"
Enter
Sleep 1s
Type "\conninfo"
Enter
Sleep 2s
Type "\q"
Enter
Ctrl+D
Sleep 500ms
Ctrl+D
Sleep 3s
Tab
Sleep 3s
Tab
Sleep 3s
Tab
Sleep 3s
Tab
Sleep 5s
