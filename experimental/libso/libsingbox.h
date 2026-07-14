/* libsingbox.h — sing-box C shared-library API
 *
 * Build artifacts: libsingbox-<arch>.so + this header.
 * Link with -lsingbox or load dynamically via dlopen / ctypes / FFI.
 *
 * Thread safety: singbox_start / singbox_stop acquire a global
 * mutex.  Once started the proxy runs in background goroutines;
 * callers can interact through the exported functions below.
 *
 * Memory: every function that returns char* allocates with malloc.
 * The caller MUST free the returned pointer with singbox_free_string().
 */

#ifndef LIBSINGBOX_H
#define LIBSINGBOX_H

#ifdef __cplusplus
extern "C" {
#endif

/* Return the sing-box core version (e.g. "1.12.0").  Free with singbox_free_string(). */
extern char *singbox_version(void);

/* Start a sing-box instance with the given JSON config.
 * Returns NULL on success; an error string on failure.  Free with singbox_free_string(). */
extern char *singbox_start(const char *config_json);

/* Stop the running instance.  Returns NULL on success, error string on failure.
 * Free with singbox_free_string(). */
extern char *singbox_stop(void);

/* Returns 1 if sing-box is running, 0 otherwise. */
extern int singbox_is_running(void);

/* Validate config without starting. Returns NULL if valid, error string otherwise.
 * Free with singbox_free_string(). */
extern char *singbox_check_config(const char *config_json);

/* Pretty-print a config JSON.  Returns formatted string or error.  Free with singbox_free_string(). */
extern char *singbox_format_config(const char *config_json);

/* Free a string returned by any singbox_* function.  Safe to call with NULL. */
extern void singbox_free_string(char *s);

#ifdef __cplusplus
}
#endif

#endif /* LIBSINGBOX_H */
