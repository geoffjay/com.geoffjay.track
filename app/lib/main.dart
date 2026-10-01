/// Mobile client for the com.geoffjay.track personal health and fitness
/// API. Authentication is a single API bearer token: the app shows a
/// token-entry gate until one is stored, then connects automatically. No
/// other auth method is supported.
library;

import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'src/app_state.dart';
import 'src/screens/home_shell.dart';
import 'src/screens/login.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(TrackApp(state: AppState(await SharedPreferences.getInstance())));
}

class TrackApp extends StatelessWidget {
  final AppState state;

  const TrackApp({super.key, required this.state});

  @override
  Widget build(BuildContext context) {
    return AppStateScope(
      notifier: state,
      child: ListenableBuilder(
        listenable: state,
        builder: (context, _) => MaterialApp(
          title: 'Track',
          debugShowCheckedModeBanner: false,
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(
              seedColor: const Color(0xFF2E7D32),
            ),
          ),
          darkTheme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(
              seedColor: const Color(0xFF2E7D32),
              brightness: Brightness.dark,
            ),
          ),
          themeMode: ThemeMode.system,
          home:
              state.hasToken ? const HomeShell() : const LoginScreen(),
        ),
      ),
    );
  }
}