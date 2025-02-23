ps aux | grep goDoChores | awk '{print $2}' | xargs kill
cd /home/green_family/chores
rm goDoChores
cp /home/green_family/goDoChores .
nohup ./goDoChores > debug.log 2>&1 &