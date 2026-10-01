/// Login gate: server URL + API token entry. Shown when no token is stored.
library;

import 'package:flutter/material.dart';

import '../api_client.dart';
import '../app_state.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _serverCtrl = TextEditingController();
  final _tokenCtrl = TextEditingController();
  bool _busy = false;
  String? _error;

  bool _seeded = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // Inherited lookups are not allowed in initState; seed once here.
    if (!_seeded) {
      _seeded = true;
      _serverCtrl.text = AppStateScope.of(context).baseUrl;
    }
  }

  @override
  void dispose() {
    _serverCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final url = _serverCtrl.text.trim();
    final token = _tokenCtrl.text.trim();

    // Verify the token against the API before persisting it: a typo would
    // otherwise leave the user stuck on broken screens until sign-out.
    final probe = ApiClient(baseUrl: url, token: token);
    try {
      await probe.listMetrics();
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.status == 401
            ? 'Token rejected — check the value and server address.'
            : e.toString();
      });
      return;
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = 'Could not reach $url: $e';
      });
      return;
    }
    if (!mounted) return;
    await AppStateScope.of(context).saveCredentials(url, token);
    if (!mounted) return;
    setState(() => _busy = false);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Form(
                key: _formKey,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Icon(Icons.monitor_heart_outlined,
                        size: 64, color: theme.colorScheme.primary),
                    const SizedBox(height: 12),
                    Text('Track',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.headlineMedium),
                    const SizedBox(height: 24),
                    TextFormField(
                      controller: _serverCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Server URL',
                        hintText: 'https://…',
                        prefixIcon: Icon(Icons.dns_outlined),
                        border: OutlineInputBorder(),
                      ),
                      keyboardType: TextInputType.url,
                      autocorrect: false,
                      autofillHints: const [AutofillHints.url],
                      validator: (v) {
                        final uri = Uri.tryParse(v?.trim() ?? '');
                        if (uri == null || !uri.hasScheme || !(uri.isScheme('http') || uri.isScheme('https'))) {
                          return 'Enter a valid http(s) URL';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 16),
                    TextFormField(
                      controller: _tokenCtrl,
                      decoration: const InputDecoration(
                        labelText: 'API token',
                        helperText:
                            'Create one on the server\u2019s Settings page',
                        prefixIcon: Icon(Icons.key_outlined),
                        border: OutlineInputBorder(),
                      ),
                      obscureText: true,
                      autocorrect: false,
                      autofillHints: const [AutofillHints.password],
                      validator: (v) =>
                          (v == null || v.trim().isEmpty) ? 'Enter the token' : null,
                    ),
                    const SizedBox(height: 20),
                    if (_error != null)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: Text(
                          _error!,
                          style: TextStyle(color: theme.colorScheme.error),
                          textAlign: TextAlign.center,
                        ),
                      ),
                    FilledButton.icon(
                      onPressed: _busy ? null : _submit,
                      icon: _busy
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child:
                                  CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.login),
                      label: const Text('Connect'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}