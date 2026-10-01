/// App-wide state: the persisted API token + server URL, and the derived
/// [ApiClient]. The app shows the token-entry gate until a token is stored;
/// once present, the main UI connects automatically.
library;

import 'dart:async';
import 'package:flutter/widgets.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'api_client.dart';

/// Sentinels so listeners can react to auth loss without string compares.
class AuthLostEvent {
  final String reason;
  const AuthLostEvent(this.reason);
}

/// Provides [AppState] to descendants: `AppStateScope.of(context)`.
class AppStateScope extends InheritedNotifier<AppState> {
  const AppStateScope({
    super.key,
    required super.notifier,
    required super.child,
  });

  static AppState of(BuildContext context) {
    final scope =
        context.dependOnInheritedWidgetOfExactType<AppStateScope>();
    assert(scope != null, 'AppStateScope missing from the tree');
    return scope!.notifier!;
  }
}

class AppState extends ChangeNotifier {
  static const _kToken = 'api_token';
  static const _kBaseUrl = 'base_url';

  /// Default server address (the deployed instance); override on the login
  /// screen for local development.
  static const defaultBaseUrl = 'https://rowing-miles.fly.dev';

  final SharedPreferences _prefs;
  final _authLostController = StreamController<AuthLostEvent>.broadcast();

  String? _token;
  String _baseUrl = defaultBaseUrl;
  ApiClient? _client;

  /// Fires when a 401 forces the user back to the token-entry screen.
  Stream<AuthLostEvent> get onAuthLost => _authLostController.stream;

  AppState(this._prefs) {
    _token = _prefs.getString(_kToken);
    final stored = _prefs.getString(_kBaseUrl);
    if (stored != null && stored.isNotEmpty) _baseUrl = stored;
    _client = _buildClient();
  }

  String? get token => _token;
  String get baseUrl => _baseUrl;
  bool get hasToken => _token != null && _token!.isNotEmpty;

  ApiClient? get client => _client;

  ApiClient? _buildClient() {
    if (!hasToken) return null;
    return ApiClient(baseUrl: _baseUrl, token: _token!);
  }

  /// Stores the token and base URL persistently, then rebuilds the client.
  Future<void> saveCredentials(String baseUrl, String token) async {
    final trimmedBase = baseUrl.trim();
    final trimmedToken = token.trim();
    await _prefs.setString(_kBaseUrl, trimmedBase);
    await _prefs.setString(_kToken, trimmedToken);
    _baseUrl = trimmedBase;
    _token = trimmedToken;
    _client = _buildClient();
    notifyListeners();
  }

  /// Wipes the stored token (settings "sign out"), returning to the gate.
  Future<void> clearToken() async {
    await _prefs.remove(_kToken);
    _token = null;
    _client = null;
    notifyListeners();
  }

  /// Called when the API rejects the stored token (401): clears state so
  /// the gate reappears, and emits the event so open snackbars can
  /// explain why.
  Future<void> handleAuthLost(String reason) async {
    await _prefs.remove(_kToken);
    _token = null;
    _client = null;
    notifyListeners();
    _authLostController.add(AuthLostEvent(reason));
  }

  @override
  void dispose() {
    _authLostController.close();
    super.dispose();
  }
}