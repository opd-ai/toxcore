When declaring network variables, always use interface types:
 - never use net.UDPAddr or net.TCPAddr
 - never use net.UDPConn, use net.PacketConn instead
 - never use net.TCPConn, use net.Conn instead
 - never use net.UDPListener net.TCPLisenter, use net.Listener instead