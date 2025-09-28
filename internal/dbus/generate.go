package dbusgen

//go:generate go tool github.com/amenzhinsky/dbus-codegen-go -camelize -output manager.go $SYSTEMD_DBUS_INTERFACE_DIR/org.freedesktop.systemd1.Manager.xml $SYSTEMD_DBUS_INTERFACE_DIR/org.freedesktop.login1.Manager.xml
