
Discord Bot - one exposed port
    Accepts incoming from server wrappers

Server Wrapper - N
    Network address of bot 
    Port to request

Bot needs to know 
* Server
    * Server Name / Modpack
    * Number of Players
    * Active Time
    * Status - {Not Running, Starting Up, Running, Disconnected}
    * Memory Usage
    * Data Save
    * Time from last connection
    * TPS
    * Connection/Socket

* Meta
    * Admins
    * Map of servers - keep info for a certain amount of time, if reconnected 
    * Commands that can run against a specific server
    * Commands that can run ON a server
