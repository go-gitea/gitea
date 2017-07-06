void _unlock_notify_callback(void *arg, int argc)
{
  extern void unlock_notify_callback(void *, int);
  unlock_notify_callback(arg, argc);
}
