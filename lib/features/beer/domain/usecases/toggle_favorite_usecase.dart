import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../repositories/i_user_preferences_repository.dart';
import '../../../../core/usecase/usecase.dart';

class ToggleFavoriteParams {
  final int beerId;
  const ToggleFavoriteParams({required this.beerId});
}

class ToggleFavoriteUseCase implements UseCase<ToggleFavoriteParams, bool> {
  ToggleFavoriteUseCase({required this.preferencesRepository});

  final IUserPreferencesRepository preferencesRepository;

  @override
  Future<Either<Failure, bool>> call(ToggleFavoriteParams params) async {
    final toggleResult = await preferencesRepository.toggleFavorite(params.beerId);
    return toggleResult.match(
      (l) => Left(l),
      (_) async {
        final favsRes = await preferencesRepository.getFavoriteIds();
        return favsRes.match(
          (l2) => Left(l2),
          (ids) => Right(ids.contains(params.beerId)),
        );
      },
    );
  }
}
