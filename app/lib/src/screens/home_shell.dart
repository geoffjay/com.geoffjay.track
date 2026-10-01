/// Bottom-navigation shell hosting the four feature tabs. Each tab keeps
/// its own Navigator so push/pop state (history screens, catalogs) is
/// preserved when switching tabs.
library;

import 'package:flutter/material.dart';

import 'fasts.dart';
import 'meals.dart';
import 'measurements.dart';
import 'settings.dart';
import 'workouts.dart';

class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;

  static const _tabs = [
    MeasurementsScreen(),
    MealsScreen(),
    FastsScreen(),
    WorkoutsScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Track'),
        actions: [
          IconButton(
            icon: const Icon(Icons.settings_outlined),
            tooltip: 'Settings',
            onPressed: () => showSettingsSheet(context),
          ),
        ],
      ),
      body: IndexedStack(
        index: _index,
        children: [
          for (final (i, tab) in _tabs.indexed)
            Navigator(
              key: ValueKey('tab-$i'),
              onGenerateRoute: (settings) => MaterialPageRoute(
                builder: (context) => tab,
              ),
            ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: const [
          NavigationDestination(
              icon: Icon(Icons.monitor_weight_outlined),
              selectedIcon: Icon(Icons.monitor_weight),
              label: 'Measure'),
          NavigationDestination(
              icon: Icon(Icons.restaurant_outlined),
              selectedIcon: Icon(Icons.restaurant),
              label: 'Meals'),
          NavigationDestination(
              icon: Icon(Icons.timer_outlined),
              selectedIcon: Icon(Icons.timer),
              label: 'Fasts'),
          NavigationDestination(
              icon: Icon(Icons.fitness_center_outlined),
              selectedIcon: Icon(Icons.fitness_center),
              label: 'Workouts'),
        ],
      ),
    );
  }
}